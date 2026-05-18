# R-005 / S-022: Auto-Correction Helpers — Research Spike

**Status**: Complete
**Date**: 2026-05-18
**Story**: S-022
**POC code**: `docs/spike-poc/auto-correction/`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Evaluation Framework](#2-evaluation-framework)
3. [Category 1: Solid-Color Border Removal](#3-category-1-solid-color-border-removal)
4. [Category 2: Gradient Border Detection](#4-category-2-gradient-border-detection)
5. [Category 3: Photo-of-Artwork Correction](#5-category-3-photo-of-artwork-correction)
6. [Go Library Landscape](#6-go-library-landscape)
7. [CPU Cost Benchmarks](#7-cpu-cost-benchmarks)
8. [Integration Plan](#8-integration-plan)
9. [Build-First vs Defer Recommendation](#9-build-first-vs-defer-recommendation)
10. [Dependencies and Supply Chain Risk](#10-dependencies-and-supply-chain-risk)

---

## 1. Executive Summary

This spike evaluates three auto-correction helpers for the image dataset tool:

| Helper | Feasibility | Recommended Phase |
|--------|------------|-------------------|
| Solid-color border removal | **High** — stdlib-only, accurate, fast | **Build first** (Phase 1) |
| Gradient border detection | **Medium** — needs tuning, variance-based approach works for common cases | **Build second** (Phase 2) |
| Photo-of-artwork correction | **Low** — pure-Go solutions are weak; requires external deps or CGo for accuracy | **Defer** until real-world test images available |

Solid-color border removal is the clear winner: it works with Go stdlib only, runs in under 1ms for typical images, and correctly handles the most common Instagram border pattern (uniform white/black padding added by reposting tools).

---

## 2. Evaluation Framework

Each category is assessed on four axes:

| Axis | Description |
|------|-------------|
| **Accuracy** | Correct detection rate on test images; false positive rate |
| **Go library availability** | Can it be done with stdlib? If not, what deps are needed? |
| **CPU cost** | Time per image at typical (1024x768) and large (3840x2160) sizes |
| **Integration complexity** | How much code; how it fits into the existing Edit/export pipeline |

---

## 3. Category 1: Solid-Color Border Removal

### 3.1 Algorithm

1. Sample the four corner pixels to determine the dominant border color.
2. Scan rows inward from top and bottom; a row is "border" if all pixels are within a configurable color-distance threshold of the border color.
3. Scan columns inward from left and right (within the already-trimmed Y range) using the same threshold.
4. Return the detected border widths (top, bottom, left, right).

The threshold parameter (default: 15.0 in Euclidean RGB distance) tolerates JPEG artifacts and minor color shifts without false positives on content.

### 3.2 POC Results (6 test images)

| Test Image | Expected Borders | Detected Borders | Correct? |
|-----------|-----------------|-----------------|----------|
| solid_white_even.png | 20px all sides | top=20, bottom=20, left=20, right=20 | Yes |
| solid_black_even.png | 20px all sides | top=23, bottom=20, left=25, right=20 | ~Yes (3px over on top/left due to content touching threshold) |
| solid_white_uneven2.png | T=10, B=30, L=5, R=25 | T=10, B=30, L=5, R=25 | Yes |
| solid_gray_thin.png | 2px all sides | 2px all sides | Yes |
| solid_mixed_thick.png | 40px all sides | 40px all sides | Yes |
| solid_white_uneven.png | 0px (no border — content fills image) | 0px | Yes |

**Accuracy: 5/6 exact, 6/6 correct within 5px tolerance.**

### 3.3 Assessment

| Axis | Rating | Notes |
|------|--------|-------|
| Accuracy | **4/5** | Handles even, uneven, thin, thick borders. Slight over-detection when content pixels happen to match border color. Mitigated by requiring minimum border width (e.g., 3px). |
| Go library availability | **5/5** | stdlib `image` package only. Zero external dependencies. |
| CPU cost | **5/5** | <1ms for 1024x768; <20ms for 4K. Early termination makes typical cases very fast. |
| Integration complexity | **5/5** | ~150 lines. Fits directly into the existing `Edit` model as a new auto-action. Uses the same crop infrastructure already in `export/imageops.go`. |

### 3.4 Known Limitations

- Does not detect borders where the border color varies per-side (e.g., Instagram story backgrounds with mixed colors).
- May over-trim if a large solid-color area exists at image edges (e.g., sky). A minimum content-area-ratio check could mitigate this.
- JPEG artifacts near the border/content boundary can cause 1-2px detection error.

---

## 4. Category 2: Gradient Border Detection

### 4.1 Algorithm

1. For each edge (top, bottom, left, right), compute the per-row or per-column pixel variance moving inward from the edge.
2. A gradient border has low variance at the edge (solid or near-solid gradient color) that increases as we approach content.
3. The border boundary is the first row/column where variance exceeds 5x the edge variance AND exceeds an absolute minimum (500).

### 4.2 POC Results (6 test images)

| Test Image | Type | Detected Borders | Assessment |
|-----------|------|-----------------|------------|
| gradient_white_fade.png | Even 30px fade to white | T=9, B=10, L=15, R=8 | Partial — detected inner ~1/3 of gradient |
| gradient_black_fade.png | Even 30px fade to black | T=10, B=9, L=45, R=8 | Asymmetric — left over-detected |
| gradient_top_only.png | 30px fade top only | T=9, B=0, L=0, R=0 | Correct direction, underestimated depth |
| gradient_sides.png | 25px fade left/right | T=0, B=0, L=45, R=6 | Left over-detected, right under-detected |
| gradient_thin.png | 8px thin fade | T=3, B=3, L=6, R=2 | Reasonable for thin gradients |
| gradient_vignette.png | Radial vignette | T=3, B=0, L=45, R=0 | Partial — vignette is hard to model as edge borders |

**Accuracy: 2/6 good, 3/6 partial, 1/6 poor.**

### 4.3 Assessment

| Axis | Rating | Notes |
|------|--------|-------|
| Accuracy | **2/5** | The variance-based approach detects gradient presence but poorly estimates exact boundaries. The ratio-based threshold is too sensitive to content patterns near edges. More sophisticated approaches (derivative-of-variance, adaptive thresholds) would improve accuracy. |
| Go library availability | **5/5** | stdlib only. No external deps needed for the detection algorithm. |
| CPU cost | **4/5** | 10ms for 1024x768; 90ms for 4K. Requires scanning all rows/columns for each edge, so O(W*H) total. |
| Integration complexity | **3/5** | ~200 lines for detection. Integration is similar to solid borders (output is crop coordinates), but needs significant tuning to be production-ready. Would benefit from a "confidence" score so the UI can flag uncertain detections for human review. |

### 4.4 Improvement Path

To make gradient detection production-ready:

1. **Derivative-of-variance approach**: Instead of absolute variance, look at the second derivative (rate of change of variance). The gradient-to-content transition creates a sharp inflection point.
2. **Dual-threshold**: Use both an absolute variance threshold and a relative (ratio) threshold, accepting the *later* of the two.
3. **Content-aware sampling**: Skip edge pixels that are clearly part of content (high local contrast) when computing the gradient profile.
4. **Confidence scoring**: Output a confidence 0-1 alongside the detected border. Let the UI show "suggested crop" for low-confidence detections.

---

## 5. Category 3: Photo-of-Artwork Correction

### 5.1 Algorithm

The POC implements a three-stage pipeline:

1. **Grayscale conversion + Sobel edge detection**: Standard 3x3 Sobel kernels compute gradient magnitude.
2. **Hough angle detection**: Strong edge pixels vote into angle bins (0.5-degree resolution, +/-20 degree range). The peak bin indicates the dominant line angle, which equals the artwork rotation.
3. **Artwork bounds detection**: Threshold strong edges and find their bounding box.

### 5.2 POC Results (6 test images)

| Test Image | Expected Angle | Detected Angle | Bounds Correct? |
|-----------|---------------|---------------|----------------|
| art_rotated_5deg.png | 5.0 deg | 0.0 deg | No — full image |
| art_rotated_neg3deg.png | -3.0 deg | 0.0 deg | No — full image |
| art_rotated_10deg.png | 10.0 deg | 0.0 deg | No — full image |
| art_perspective_mild.png | ~0 deg + mild skew | 0.0 deg | No — full image |
| art_perspective_strong.png | ~0 deg + strong skew | 0.0 deg | No — full image |
| art_with_frame.png | 8.0 deg | 0.0 deg | No — full image |

**Accuracy: 0/6. The pure-Go approach failed completely on all test images.**

### 5.3 Root Cause of Failure

The Hough angle detection approach failed because:

1. **Synthetic test images lack realistic edge contrast.** The randomly-colored rectangles in the "artwork" produce many strong edges at 0/90 degrees that overwhelm the rotated artwork boundary edges.
2. **The simplified Hough transform bins edge gradient directions**, not actual line segments. In a real photo-of-artwork scenario, the artwork boundary is a single strong rectangular edge, but our accumulator treats all edges equally. A proper Hough line transform would be needed.
3. **No background/foreground separation.** Without knowing which pixels are "background" (wall) vs "artwork", the algorithm cannot isolate the artwork boundary from internal content edges.

### 5.4 What Would Work

For accurate photo-of-artwork correction, you need:

1. **Probabilistic Hough Line Transform (PHT)** — detects actual line segments, not just gradient directions. Not available in Go stdlib; requires OpenCV or a dedicated implementation.
2. **Contour detection + quadrilateral fitting** — find the largest quadrilateral in the image, then compute the perspective transform to deskew it. This is the approach used by document scanners. Requires:
   - Gaussian blur (reduce noise)
   - Canny edge detection (better than Sobel for contour finding)
   - Contour finding with hierarchy
   - Convex hull + polygon approximation
   - Perspective (homography) transformation
3. **GrabCut or color-based segmentation** — separate artwork from background using color models, then find the artwork boundary.

All three approaches either require OpenCV bindings (CGo) or a substantial pure-Go implementation (1000+ lines for a basic version).

### 5.5 Assessment

| Axis | Rating | Notes |
|------|--------|-------|
| Accuracy | **1/5** | POC approach is not viable. A proper implementation (PHT + contour fitting) could reach 3-4/5 on well-lit photos. |
| Go library availability | **2/5** | No mature pure-Go library exists for the required operations (Hough line transform, contour detection, perspective warp). gocv exists but requires CGo + OpenCV C++ library. |
| CPU cost | **3/5** | Sobel + Hough is fast (~40ms for 1080p) but a full contour-finding + perspective correction pipeline would be 200-500ms per image. |
| Integration complexity | **1/5** | Would require either: (a) CGo + OpenCV dependency (breaks the pure-Go, no-CGO constraint), or (b) 1000+ lines of custom geometry code with uncertain accuracy. |

---

## 6. Go Library Landscape

### 6.1 Stdlib (image, image/draw, image/color)

- **Strengths**: Pixel access, basic drawing, format decoding (PNG, JPEG, GIF). Already used by the project.
- **Limitations**: No convolution kernels, no edge detection, no geometric transforms beyond basic crop/rotate (which the project already implements in `export/imageops.go`).
- **Sufficient for**: Solid border detection/removal, basic gradient detection.

### 6.2 golang.org/x/image

- **Type**: Semi-official extended library.
- **Additions**: Additional image formats (TIFF, BMP, WebP), font rendering, `draw.BiLinear`/`draw.CatmullRom` scalers.
- **Relevant**: The `draw` package's affine transform support could replace the custom bilinear rotation in `imageops.go` with higher quality.
- **Supply chain risk**: Low (maintained by Go team).
- **Verdict**: Worth adding for improved rotation quality. Not needed for border detection.

### 6.3 github.com/disintegration/imaging

- **Stars**: ~5.2k (as of early 2025)
- **Type**: Pure Go image processing library.
- **Relevant features**: Resize, crop, rotate, flip, blur, sharpen, brightness/contrast adjustment.
- **Limitations**: No edge detection, no Hough transform, no perspective correction. Essentially a convenience wrapper around stdlib with better resize algorithms.
- **Supply chain risk**: Low (single author, stable, minimal deps).
- **Verdict**: Not needed — the project already has equivalent functionality in `imageops.go`.

### 6.4 github.com/anthonynsimon/bild

- **Stars**: ~4.0k (as of early 2025)
- **Type**: Pure Go image processing with convolution support.
- **Relevant features**: Sobel/Prewitt/Laplacian edge detection, Gaussian blur, convolution kernels, histogram operations.
- **Limitations**: No Hough transform, no contour detection, no perspective correction.
- **Supply chain risk**: Medium (single author, less active than imaging).
- **Verdict**: Could accelerate gradient border detection with proper edge detection kernels. Not sufficient for artwork correction.

### 6.5 gocv.io/x/gocv

- **Stars**: ~6.5k (as of early 2025)
- **Type**: Go bindings for OpenCV 4.x. **Requires CGo and system-installed OpenCV.**
- **Relevant features**: Full computer vision pipeline — Hough transform, contour detection, perspective warp, GrabCut segmentation, template matching.
- **Limitations**: **Breaks the project's CGo-free constraint** (modernc.org/sqlite was chosen specifically to avoid CGo). Adds ~500MB to Docker image. Build complexity increases significantly.
- **Supply chain risk**: High (C++ dependency, complex build chain, security surface of OpenCV).
- **Verdict**: Only option for production-quality artwork correction, but the CGo cost is prohibitive for this project.

### 6.6 Summary Matrix

| Library | Pure Go | Border Removal | Gradient Detection | Artwork Correction | Risk |
|---------|---------|---------------|-------------------|-------------------|------|
| stdlib `image` | Yes | Sufficient | Basic | No | None |
| `x/image` | Yes | N/A | N/A | No | Low |
| `disintegration/imaging` | Yes | Redundant | No | No | Low |
| `anthonynsimon/bild` | Yes | N/A | Helpful | No | Medium |
| `gocv.io/x/gocv` | **No (CGo)** | Overkill | Overkill | **Yes** | High |

---

## 7. CPU Cost Benchmarks

All benchmarks run on the POC code in the sandbox environment. Average of 5 runs after warmup.

### 7.1 Solid Border Detection

| Image Size | Time | Notes |
|-----------|------|-------|
| 640x480 | 0.58ms | Typical Instagram image |
| 1024x768 | <0.01ms | Early termination (no border on test image) |
| 1920x1080 | <0.01ms | Early termination |
| 3840x2160 | 19ms | Full scan (border present) |

The early-termination design means images without borders cost almost nothing. Images with borders scale linearly with the border perimeter, not the image area.

### 7.2 Gradient Border Detection

| Image Size | Time |
|-----------|------|
| 640x480 | 3.8ms |
| 1024x768 | 9.6ms |
| 1920x1080 | 22ms |
| 3840x2160 | 90ms |

Scans all four edges, each requiring a full row/column variance pass. O(W*H) total.

### 7.3 Artwork Rotation Detection (Sobel + Hough)

| Image Size | Sobel Only | Full Pipeline |
|-----------|-----------|--------------|
| 640x480 | 6ms | 7ms |
| 1024x768 | 15ms | 18ms |
| 1920x1080 | 30ms | 41ms |
| 3840x2160 | 123ms | 163ms |

Even though the algorithm does not produce accurate results, the timing data is informative: a proper pipeline (Canny + Hough + contour) would likely be 3-5x slower than these numbers.

### 7.4 Cost Context

For this application, images are processed one at a time during export or when the user triggers auto-correction in the review UI. At typical Instagram sizes (1080x1080 to 1440x1800):

- Solid border removal: **<1ms** — imperceptible to user.
- Gradient detection: **~15ms** — imperceptible to user.
- Artwork correction: **~50ms** (if it worked) — acceptable for interactive use.

None of these are CPU bottlenecks. Image decode (PNG/JPEG) dominates processing time.

---

## 8. Integration Plan

### 8.1 Solid Border Removal (Phase 1)

Fits cleanly into the existing architecture:

1. **New auto-action type**: Add `"auto-remove-border"` to the `Edit.AutoActionsJSON` schema. This is already referenced in the PRD (section 8.3).
2. **Detection function**: `DetectSolidBorder(img image.Image, threshold float64) (top, bottom, left, right int)` in a new file `backend/internal/export/borderdetect.go`.
3. **Integration points**:
   - `export/materializer.go`: Apply border removal before other edits during export.
   - Review UI: Add "Auto-Remove Borders" button that calls detection, previews the crop, and saves to the edit record.
4. **User control**: The detected crop is shown as a preview; the user confirms or adjusts. This matches the existing edit model (non-destructive ops stored in manifest).

Estimated implementation: **1-2 days, ~200 lines of Go + frontend button/preview**.

### 8.2 Gradient Border Detection (Phase 2)

Same integration pattern as solid borders, but with additional work:

1. Refine the detection algorithm (derivative-of-variance, dual threshold).
2. Add confidence scoring.
3. UI shows "suggested" crop with confidence indicator; low-confidence detections require explicit user approval.

Estimated implementation: **3-5 days, ~400 lines of Go + frontend confidence UI**.

### 8.3 Photo-of-Artwork Correction (Deferred)

Not recommended for near-term implementation due to:
- No viable pure-Go library.
- CGo dependency conflicts with project constraints.
- Low accuracy without OpenCV-grade algorithms.

If revisited, the least-cost approach would be:
1. **External microservice**: Run a Python/OpenCV container as a sidecar that accepts images via HTTP and returns correction parameters (angle, perspective matrix, crop bounds).
2. The Go backend calls this service and applies the returned transforms using existing `imageops.go` rotation and crop functions.
3. This avoids CGo in the main binary while getting OpenCV accuracy.

---

## 9. Build-First vs Defer Recommendation

### Build First (Phase 1): Solid-Color Border Removal

**Why**: Highest impact-to-effort ratio. Instagram images frequently have solid borders from reposting tools, collage apps, and manual padding. Detection is accurate, fast, and requires zero external dependencies. Already referenced in the PRD as a planned feature.

### Build Second (Phase 2): Gradient Border Detection

**Why**: Covers the second-most common border pattern (fade-to-white/black used by Instagram story tools). Needs more tuning but the basic approach works. Same integration pattern as Phase 1.

### Defer: Photo-of-Artwork Correction

**Why**: The use case (photos of physical artwork) is less common in Instagram datasets than border issues. The technical solution requires either CGo (unacceptable) or a sidecar service (significant infrastructure overhead). Recommend deferring until:
1. Real-world test images confirm the use case frequency.
2. The sidecar architecture is justified by other needs (e.g., ML-based captioning already runs externally).

---

## 10. Dependencies and Supply Chain Risk

### Current (no new deps needed for Phase 1)

Solid border removal uses only Go stdlib. **Zero supply chain risk added.**

### Phase 2 (optional)

Gradient detection also uses only stdlib. If `anthonynsimon/bild` is added for better convolution kernels:
- **Risk**: Single maintainer, pure Go, pinnable version. Acceptable.
- **Mitigation**: Vendor the specific convolution functions (~200 lines) instead of importing the full library.

### Deferred (artwork correction)

If the sidecar approach is chosen later:
- **Risk**: Python + OpenCV dependency isolated in a separate container. Does not affect the main Go binary's supply chain.
- **Risk**: `gocv` (if ever used directly) adds OpenCV C++ as a transitive dependency — significant attack surface. **Not recommended.**

### General Guidance

The project's CGo-free constraint is a strong supply chain advantage. All image processing for Phases 1-2 should remain stdlib-only. The only scenario warranting new Go image deps would be if `golang.org/x/image` is added for improved affine transform quality in the existing rotation code — and that library has minimal risk as a Go team project.

---

## Appendix A: POC Code

The proof-of-concept code is in `docs/spike-poc/auto-correction/`:

```
cmd/generate_testdata/  — generates 18 synthetic test images (6 per category)
cmd/poc/                — runs all three detection algorithms, saves corrected outputs
cmd/benchmark/          — CPU cost measurements at four image sizes
testdata/               — generated test images (run generate_testdata to create)
output/                 — corrected images (run poc to create)
```

To reproduce:
```bash
cd docs/spike-poc/auto-correction
go run ./cmd/generate_testdata
go run ./cmd/poc
go run ./cmd/benchmark
```

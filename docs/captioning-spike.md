# CAP-001: Vision Model Captioning Evaluation

**Status**: Draft (autonomous research phase complete)
**Date**: 2026-05-11
**Story**: S-025
**Research basis**: Provider documentation, community benchmarks, and known best practices as of May 2025. Pricing, rate limits, and model availability should be verified against current provider pages before implementation.

---

## Table of Contents

1. [Evaluation Criteria](#1-evaluation-criteria)
2. [Provider Profiles](#2-provider-profiles)
3. [Recommended Default Prompt Template](#3-recommended-default-prompt-template)
4. [Starting Recommendation](#4-starting-recommendation)
5. [Provider-Specific Quirks](#5-provider-specific-quirks-for-captioner-interface)
6. [Gold Set Design](#6-gold-set-design-for-interactive-phase)
7. [Appendix: Pricing Comparison Table](#7-appendix-pricing-comparison-table)

---

## 1. Evaluation Criteria

Each criterion is scored 1-5 and weighted to reflect priorities for SD/Flux LoRA training data.

| Criterion                  | Weight | Description |
|----------------------------|--------|-------------|
| **Accuracy**               | 25%    | Caption correctly describes visible elements (subject identity, actions, objects, setting). No hallucinated details. |
| **Detail level**           | 25%    | Sufficient detail for SD/Flux training without over-describing. Target: 2-4 sentences covering subject appearance, pose, setting, lighting. Not so long it dilutes key features. |
| **Consistency**            | 20%    | Similar structure and style across different images. Captions follow a predictable pattern (e.g., subject first, then setting, then mood/style). Critical for LoRA training where inconsistent captions confuse the model. |
| **Natural language quality**| 15%   | Reads as fluent English prose. Not booru-tag lists, not robotic enumerations. Avoids filler words and repetitive phrases. |
| **Cost per image**         | 10%    | API pricing for a typical captioning call (one image ~1024px, ~200-token output). |
| **Latency**                | 5%     | Time per caption request. Less critical since captioning is batched asynchronously. |

### Scoring rubric

- **5**: Excellent. Best-in-class for this criterion.
- **4**: Good. Minor issues that rarely affect training quality.
- **3**: Acceptable. Noticeable limitations but workable.
- **2**: Below average. Frequent issues that may affect training.
- **1**: Poor. Unsuitable for production use.

### Why these weights

Accuracy and detail are weighted highest because incorrect or insufficient captions directly degrade LoRA training quality. Consistency is next because uniform caption structure helps the model learn associations reliably. Natural language quality matters for readability during manual review but has less direct impact on training. Cost and latency are lowest because captioning is a batch operation done once per dataset; even expensive providers are affordable at 30-500 images per dataset.

---

## 2. Provider Profiles

### 2.1 Anthropic Claude

#### Available Vision Models

| Model | Context Window | Max Output | Vision Support |
|-------|---------------|------------|----------------|
| **Claude Sonnet 4** (claude-sonnet-4-20250514) | 200K tokens | 16,384 tokens | Yes — images in messages API |
| **Claude Opus 4** (claude-opus-4-20250514) | 200K tokens | 32,000 tokens | Yes — images in messages API |
| Claude Haiku 3.5 (claude-3-5-haiku-20241022) | 200K tokens | 8,192 tokens | Yes — images in messages API |

**Note**: Claude 4 Sonnet and Opus were released May 2025. Verify exact model IDs against the Anthropic docs at implementation time.

#### API Format Requirements

- **Image encoding**: Base64-encoded in `image` content blocks, or URL references.
- **Supported formats**: JPEG, PNG, GIF, WebP.
- **Size limits**: Images are resized server-side. Max ~5MB per image. Recommended: send images at 1024px on the longest edge to control token cost. Each image consumes tokens based on its pixel count (roughly 1 token per 750 pixels after internal tiling).
- **Image token cost**: A 1024x1024 image costs approximately 1,334 tokens. A 1568x1568 image (max tile size) costs ~1,600 tokens.

#### Rate Limits (Tier 1 — new accounts)

| Limit Type | Claude Sonnet 4 | Claude Opus 4 |
|------------|-----------------|---------------|
| RPM        | 50              | 50            |
| TPM        | 40,000          | 40,000        |
| Daily      | Varies by tier  | Varies by tier |

Higher tiers (Tier 2-4) unlock significantly more capacity. Verify current tier limits at https://docs.anthropic.com/en/docs/about-claude/models.

#### Pricing (as of May 2025)

| Model | Input (per 1M tokens) | Output (per 1M tokens) |
|-------|----------------------|------------------------|
| Claude Sonnet 4 | $3.00 | $15.00 |
| Claude Opus 4   | $15.00 | $75.00 |
| Claude Haiku 3.5 | $0.80 | $4.00 |

**Estimated cost per caption** (1024x1024 image + ~100-token prompt + ~200-token output):
- Sonnet 4: ~$0.0074 (~1,500 input tokens x $3/1M + 200 output tokens x $15/1M)
- Opus 4: ~$0.0375
- Haiku 3.5: ~$0.002

#### Strengths for Captioning
- Excellent instruction following and natural language quality.
- Very low hallucination rate on visual description tasks.
- Good at maintaining consistent output structure when prompted.
- Handles diverse image types well (portraits, landscapes, indoor/outdoor).

#### Weaknesses
- Higher cost than competitors (especially Opus).
- Image token accounting is somewhat opaque (tiling-based).

---

### 2.2 OpenAI GPT-4o Family

#### Available Vision Models

| Model | Context Window | Max Output | Vision Support |
|-------|---------------|------------|----------------|
| **GPT-4o** (gpt-4o) | 128K tokens | 16,384 tokens | Yes — images in chat completions |
| **GPT-4o-mini** (gpt-4o-mini) | 128K tokens | 16,384 tokens | Yes — images in chat completions |

#### API Format Requirements

- **Image encoding**: Base64-encoded data URLs in `image_url` content parts, or direct URLs.
- **Supported formats**: JPEG, PNG, GIF, WebP.
- **Detail parameter**: `detail: "low"` (fixed 85 tokens) or `detail: "high"` (tiled, variable cost). For captioning, `detail: "high"` is recommended.
- **Size limits**: Max 20MB per image. Images larger than 2048px on the long edge are resized. High-detail tiles are 512x512, each costing 170 tokens plus a base 85 tokens.
- **Image token cost** (high detail): A 1024x1024 image uses ~765 tokens (4 tiles x 170 + 85).

#### Rate Limits (Tier 1)

| Limit Type | GPT-4o | GPT-4o-mini |
|------------|--------|-------------|
| RPM        | 500    | 500         |
| TPM        | 30,000 | 200,000     |

Higher tiers are available. RPM limits are generally more generous than Anthropic at equivalent tiers.

#### Pricing (as of May 2025)

| Model | Input (per 1M tokens) | Output (per 1M tokens) |
|-------|----------------------|------------------------|
| GPT-4o      | $2.50 | $10.00 |
| GPT-4o-mini | $0.15 | $0.60  |

**Estimated cost per caption** (1024x1024 high-detail + ~100-token prompt + ~200-token output):
- GPT-4o: ~$0.0042 (~850 input tokens x $2.50/1M + 200 output tokens x $10/1M)
- GPT-4o-mini: ~$0.00027

#### Strengths for Captioning
- GPT-4o-mini is extremely cost-effective — nearly 30x cheaper than Claude Sonnet 4.
- Good vision capabilities, competitive with Claude on description tasks.
- Higher default RPM limits than Anthropic.
- Well-documented API with mature SDK ecosystem.

#### Weaknesses
- GPT-4o-mini can be less detailed and occasionally more formulaic in descriptions.
- `detail` parameter adds complexity to token cost estimation.
- Occasionally produces more "list-like" descriptions compared to Claude's prose style.

---

### 2.3 Google Gemini

#### Available Vision Models

| Model | Context Window | Max Output | Vision Support |
|-------|---------------|------------|----------------|
| **Gemini 2.5 Flash** | 1M tokens | 65,536 tokens | Yes — multimodal |
| **Gemini 2.5 Pro** | 1M tokens | 65,536 tokens | Yes — multimodal |

#### API Format Requirements

- **Image encoding**: Base64-encoded `inlineData` parts, or `fileData` references (via File API for larger files).
- **Supported formats**: JPEG, PNG, GIF, WebP, HEIC/HEIF.
- **Size limits**: Up to 20MB per inline image. File API supports up to 2GB.
- **Image token cost**: Gemini uses a different token counting approach. A typical image costs ~258 tokens regardless of resolution (images are resized internally).
- **HEIC support**: Native HEIC support is a notable advantage for iPhone-sourced photos.

#### Rate Limits

| Limit Type | Gemini 2.5 Flash | Gemini 2.5 Pro |
|------------|------------------|----------------|
| RPM        | 2,000 (free) / 2,000 (paid) | 150 (free) / 1,000 (paid) |
| TPM        | 4M (free) / 4M (paid)      | 2M (free) / 2M (paid)    |

Gemini has the most generous rate limits of any major cloud provider. The free tier is substantial.

#### Pricing (as of May 2025)

| Model | Input (per 1M tokens) | Output (per 1M tokens) |
|-------|----------------------|------------------------|
| Gemini 2.5 Flash | $0.15 (<=200K ctx) | $0.60 (<=200K ctx) |
| Gemini 2.5 Pro   | $1.25 (<=200K ctx) | $10.00 (<=200K ctx) |

**Note**: Gemini offers a free tier with lower rate limits suitable for development/testing.

**Estimated cost per caption** (~350 input tokens with image + ~200-token output):
- Gemini 2.5 Flash: ~$0.00017
- Gemini 2.5 Pro: ~$0.0024

#### Strengths for Captioning
- Gemini 2.5 Flash is the cheapest cloud option — nearly 45x cheaper than Claude Sonnet 4.
- Generous rate limits enable high-throughput batch processing.
- Native HEIC support (relevant for iPhone-sourced Instagram photos).
- Excellent vision quality — Gemini 2.5 Pro scores competitively on vision benchmarks.
- Free tier available for development and testing.

#### Weaknesses
- Google API library (`google-genai` / REST) differs from OpenAI-compatible format — requires a dedicated `Captioner` implementation.
- Gemini can occasionally be verbose or include safety-related caveats in output.
- "Thinking" tokens in 2.5 models may inflate cost if not managed via `thinkingBudget` parameter.
- API stability has historically been less predictable than OpenAI/Anthropic (model deprecation cycles).

---

### 2.4 xAI Grok

#### Available Vision Models

| Model | Context Window | Max Output | Vision Support |
|-------|---------------|------------|----------------|
| **Grok 2 Vision** (grok-2-vision-1212) | 32K tokens | 4,096 tokens | Yes — OpenAI-compatible API |

**Note**: xAI's model lineup evolves quickly. Verify current models at https://docs.x.ai/docs.

#### API Format Requirements

- **API compatibility**: OpenAI-compatible chat completions API (`https://api.x.ai/v1/chat/completions`).
- **Image encoding**: Same as OpenAI — base64 data URLs or image URLs in `image_url` content parts.
- **Supported formats**: JPEG, PNG, GIF, WebP.
- **Size limits**: Similar to OpenAI constraints.

#### Rate Limits

| Limit Type | Grok 2 Vision |
|------------|---------------|
| RPM        | ~60 (varies by tier) |
| TPM        | ~100,000 |

Rate limits are less well-documented than competitors. Verify current limits.

#### Pricing (as of May 2025)

| Model | Input (per 1M tokens) | Output (per 1M tokens) |
|-------|----------------------|------------------------|
| Grok 2 Vision | $2.00 | $10.00 |

**Estimated cost per caption**: ~$0.004 (comparable to GPT-4o).

#### Strengths for Captioning
- OpenAI-compatible API means it can reuse the same `Captioner` implementation as OpenAI.
- Competitive pricing.

#### Weaknesses
- Smaller context window (32K) — not a concern for single-image captioning but limits batch approaches.
- Less mature API documentation and ecosystem.
- Vision capabilities are less well-benchmarked than GPT-4o or Claude.
- Lower max output tokens (4K) — sufficient for captions but limiting.
- Smaller community and fewer captioning-specific evaluations.

---

### 2.5 Local OAI-Compatible Models

#### Recommended Open Vision Models

| Model | Parameters | VRAM Required | Quality Tier |
|-------|-----------|---------------|--------------|
| **Qwen2.5-VL-72B** | 72B | ~40GB (Q4) | High — competitive with GPT-4o |
| **Qwen2.5-VL-7B** | 7B | ~6GB (Q4) | Medium — good for cost-sensitive batching |
| **InternVL2.5-78B** | 78B | ~45GB (Q4) | High — strong on detail and accuracy |
| **InternVL2.5-8B** | 8B | ~6GB (Q4) | Medium — solid for its size |
| **LLaVA-OneVision-72B** | 72B | ~40GB (Q4) | High — good natural language |
| **Pixtral-Large-124B** | 124B | ~70GB (Q4) | Very high — Mistral's vision model |

#### Serving Options

- **llama.cpp** (`llama-server`): Lightweight C++ inference server. Supports GGUF quantized models. OpenAI-compatible `/v1/chat/completions` endpoint. Vision support via multimodal projectors (mmproj files). Best for single-GPU setups.
- **LM Studio**: GUI application with built-in OpenAI-compatible server. Easy model management. Supports vision models with appropriate mmproj files.
- **vLLM**: High-throughput serving with continuous batching. Best for multi-GPU setups. Supports AWQ/GPTQ quantization. OpenAI-compatible API.
- **Ollama**: Simplified model management. Built-in OpenAI-compatible API. Supports vision models natively (e.g., `llava`, `qwen2.5-vl`).

#### API Format Requirements

All local servers expose an OpenAI-compatible chat completions API:
- **Endpoint**: `http://localhost:<port>/v1/chat/completions`
- **Image encoding**: Base64 data URLs in `image_url` content parts (same as OpenAI).
- **Supported formats**: Depends on the model's vision encoder — typically JPEG, PNG.
- **Size limits**: Constrained by model context length and available VRAM. Most models resize images internally.

#### Rate Limits

- No external rate limits — throughput is hardware-bound.
- A typical consumer GPU (RTX 4090, 24GB VRAM) runs a 7B vision model at ~10-30 tokens/sec output.
- A 72B model on dual 4090s might achieve ~5-15 tokens/sec.
- Set `rpm: 0` and `tpm: 0` in config.yaml for local providers (unlimited).

#### Pricing

- **Hardware cost only**: no per-token charges.
- Electricity cost is negligible for small datasets (30-500 images).
- A 500-image batch at ~10 sec/caption = ~83 minutes on a 7B model.

#### Strengths
- Zero marginal cost per caption.
- Complete privacy — images never leave the local machine.
- No rate limits or API quotas.
- Qwen2.5-VL-72B specifically has shown competitive results with GPT-4o on vision benchmarks (MMMU, DocVQA, etc.).

#### Weaknesses
- Requires significant VRAM for high-quality models (72B+ need 40GB+).
- Slower than cloud APIs (especially for larger models).
- Setup complexity — need to download models, configure mmproj files, manage VRAM.
- Quality of 7B models is noticeably below cloud providers.
- Less consistent output structure compared to instruction-tuned cloud models.
- Vision support in llama.cpp requires specific mmproj files matched to the model.

---

## 3. Recommended Default Prompt Template

### Template

```
Describe this image in detail for use as a training caption. Write in natural, flowing English prose (not tags or bullet points).

Structure your description as follows:
1. Start with the main subject — describe their appearance, clothing, expression, and pose.
2. Describe the setting and environment.
3. Note the lighting, color palette, and overall mood.
4. Mention the photographic style or composition if notable.

Keep the description between 2-4 sentences. Be precise and specific rather than vague. Do not speculate about emotions, intentions, or narrative context beyond what is visually evident. Do not include any preamble or meta-commentary — output only the description.
```

### Design Choices Explained

1. **"Natural, flowing English prose"**: Explicitly excludes booru-tag style (`1girl, long hair, standing, outdoor`) which is common in anime/illustration training but counterproductive for photographic LoRA training. SD/Flux text encoders (T5-XXL for Flux, CLIP for SD) parse natural language better than comma-separated tags.

2. **Structured order (subject > setting > lighting > style)**: Provides a consistent anchor across all captions. LoRA training benefits from the subject description appearing early and consistently, as the model learns to associate the trigger word with these descriptions. Having a predictable structure also makes manual review much faster.

3. **"2-4 sentences"**: This length sweet spot provides enough detail for the model to learn meaningful associations without flooding the text encoder's effective attention window. For SDXL/Flux LoRA training, captions of 50-150 tokens work best. Shorter captions under-specify; longer captions dilute the per-token weight of each concept.

4. **"Do not speculate"**: Vision models sometimes infer emotional states or narrative context not visible in the image. This confuses training by introducing concepts the visual doesn't support.

5. **"Do not include any preamble"**: Prevents models from prepending "The image shows..." or "In this photograph..." which wastes tokens and adds no training value.

6. **No trigger word in template**: The trigger word (e.g., `sks person`, `ohwx man`) is prepended by the application at export time, not included in the caption prompt. This keeps captions reusable across different LoRA configurations.

### Example Output

For a portrait photo of a person at a cafe:

> A young woman with shoulder-length dark hair sits at a small round table in an outdoor cafe, wearing a cream-colored linen blouse and gold hoop earrings. The table holds a white ceramic coffee cup and a folded newspaper. Warm afternoon sunlight filters through a green canvas awning, casting soft dappled shadows across the scene. The composition is shot at eye level with a shallow depth of field that blurs the cobblestone street and passersby in the background.

### Prompt Variants

For specific use cases, the template can be adapted:

**Minimal variant** (for when shorter captions are preferred):
```
Describe this image in one concise sentence for use as a training caption. Focus on the subject's appearance, pose, and setting. Write in natural English prose. Output only the description.
```

**Detail-heavy variant** (for complex scenes):
```
Describe this image in detail for use as a training caption. Write in natural, flowing English prose.

Cover all of the following:
1. Main subject — appearance, clothing, accessories, expression, pose, body position.
2. Setting — location type, key objects, background elements.
3. Lighting — direction, quality (hard/soft), color temperature, shadows.
4. Color palette — dominant colors, contrast level.
5. Photographic style — framing, angle, depth of field, lens characteristics.

Be precise and specific. Do not speculate beyond what is visible. Output only the description (3-5 sentences).
```

---

## 4. Starting Recommendation

### Primary: Claude Sonnet 4

**Recommended as the Day-1 default provider.**

| Factor | Assessment |
|--------|-----------|
| Accuracy | Excellent — low hallucination rate, precise visual descriptions |
| Detail | Well-calibrated — follows length instructions reliably |
| Consistency | Very good — maintains structure across diverse images |
| Natural language | Best-in-class prose quality among cloud providers |
| Cost | $0.007/image — moderate. 500 images = ~$3.70 |
| Latency | ~2-4 seconds per caption |

**Justification**:
- The PRD designates Claude as the Day-1 default. Sonnet 4 offers the best quality/cost ratio in the Claude family.
- Opus 4 is 5x more expensive with marginal quality improvement for captioning tasks — reserve for edge cases.
- Claude's instruction following is particularly strong for maintaining consistent caption structure, which is the most important factor for LoRA training quality.
- The application already integrates with Anthropic's API (PRD §8.4), minimizing implementation effort.

### Budget Alternative: Gemini 2.5 Flash

For users who need to caption large datasets (1000+ images) or want to minimize cost during iteration:
- 45x cheaper than Claude Sonnet 4.
- Quality is good but slightly less consistent in structure.
- Would require a separate `Captioner` implementation (non-OpenAI-compatible API).

### Quality Alternative: GPT-4o

If Anthropic API access is unavailable or rate-limited:
- Uses OpenAI-compatible API (shares implementation with Grok and local models).
- Competitive quality with Claude Sonnet 4.
- Slightly lower cost per caption.

### Provider Priority for Implementation

1. **Anthropic Claude** (Day-1 default per PRD)
2. **OpenAI-compatible** (covers GPT-4o, GPT-4o-mini, Grok, and all local models via a single implementation)
3. **Google Gemini** (separate implementation but best cost/throughput)

This order maximizes coverage with minimum implementation effort. Two `Captioner` implementations (Anthropic + OpenAI-compatible) cover 4 out of 5 provider categories.

---

## 5. Provider-Specific Quirks for Captioner Interface

This section documents implementation gotchas relevant to building the `Captioner` interface per PRD §8.4.

### 5.1 Anthropic Claude

- **No system prompt for vision**: Images must be in `user` message content blocks, not system prompts. The caption instruction prompt goes in the same user message or a preceding user message.
- **Image resizing**: Anthropic auto-resizes images but charges tokens based on the size received. Pre-resize to 1024px longest edge before sending to control costs.
- **Content block ordering**: Image blocks should come before text blocks in the user message for best results.
- **Stop reason**: Check for `stop_reason: "end_turn"` vs `"max_tokens"` — if max_tokens is hit, the caption was truncated.
- **Beta headers**: Some features (like token counting) require beta headers. The stable messages API is sufficient for captioning.
- **Rate limit headers**: Response includes `anthropic-ratelimit-requests-remaining` and `anthropic-ratelimit-tokens-remaining` headers — use these for proactive rate limiting rather than waiting for 429s.

### 5.2 OpenAI / OpenAI-Compatible (GPT-4o, Grok, Local)

- **`detail` parameter**: Must set `detail: "high"` for quality captioning. Default is `"auto"` which may use low detail for small images, producing poor captions.
- **`max_tokens` vs `max_completion_tokens`**: Older API versions use `max_tokens`; newer use `max_completion_tokens`. Use the appropriate field for the API version.
- **Local model differences**: Not all local models support the `detail` parameter — llama.cpp ignores it. Send appropriately-sized images (1024px) regardless.
- **Streaming**: For local models with slow inference, streaming responses can improve perceived latency and enable progress tracking. Cloud providers are fast enough that non-streaming is fine.
- **Token counting**: OpenAI returns `usage.prompt_tokens` and `usage.completion_tokens` in the response. Local servers may or may not include this — handle missing usage data gracefully.
- **Image URL vs base64**: Some local servers don't support URL-based images (no outbound network access). Always use base64 encoding for maximum compatibility.

### 5.3 Google Gemini

- **Different API format**: Uses `generateContent` endpoint with `Part` objects, not OpenAI chat completions. Requires its own `Captioner` implementation.
- **`thinkingBudget`**: Gemini 2.5 models include "thinking" tokens by default. Set `thinkingBudget: 0` in generation config to disable thinking and reduce cost/latency for captioning (which doesn't benefit from chain-of-thought).
- **Safety settings**: Gemini may refuse to caption images it deems sensitive. Override safety settings to `BLOCK_NONE` for all categories (Anthropic and OpenAI are more permissive by default for descriptive tasks).
- **`responseMimeType`**: Can be set to `text/plain` to prevent JSON wrapping.
- **Candidate count**: Always set to 1 for captioning (default may vary).
- **Model name format**: Models are referenced as `models/gemini-2.5-flash` in the API, not bare names.

### 5.4 xAI Grok

- **OpenAI-compatible**: Uses the same request format as OpenAI. Can share the OpenAI `Captioner` implementation with a different base URL (`https://api.x.ai/v1`).
- **Lower max output**: 4,096 tokens max — sufficient for captions but set `max_tokens` conservatively (512) to avoid unexpected truncation.
- **API key**: Uses `Authorization: Bearer <key>` header, same as OpenAI.

### 5.5 Local Models (llama.cpp / LM Studio / Ollama / vLLM)

- **Multimodal projector**: llama.cpp requires a separate `--mmproj` file matched to the vision model. Missing or mismatched mmproj produces garbage output without errors.
- **Context length**: Many local models default to low context lengths (2048-4096). Ensure the server is started with sufficient context for image tokens + prompt + output (recommend `--ctx-size 4096` minimum).
- **Slow cold start**: First inference after loading may be very slow (30-60s) due to model loading. Subsequent requests are faster. The `Captioner` should have a longer initial timeout.
- **Memory pressure**: Running a large vision model alongside the application itself may cause OOM. The application should detect this (connection refused / timeout) and surface a clear error rather than hanging.
- **No usage metrics**: Most local servers don't return token usage in the response. The rate limiter should fall back to estimated token counts based on prompt/response length.
- **Health check**: Implement a health check endpoint (`GET /v1/models`) before starting a batch to verify the local server is running and has a model loaded.

---

## 6. Gold Set Design (for Interactive Phase)

The gold set should contain **30 images** selected to cover the diversity dimensions that LoRA training datasets typically contain. The goal is to evaluate captioning quality across the full range of conditions the tool will encounter.

### Selection Criteria

#### Subject Diversity (10 images)
- 2x close-up portraits (face-focused, different people)
- 2x upper-body shots (casual clothing visible)
- 2x full-body shots (showing pose and outfit)
- 2x group shots (2-3 people)
- 2x non-human subjects (pet, object, food — for edge case testing)

#### Setting Diversity (8 images)
- 2x indoor (home, office)
- 2x outdoor urban (street, cafe, architecture)
- 2x outdoor nature (park, beach, forest)
- 2x studio/controlled (plain background, professional lighting)

#### Lighting Diversity (6 images)
- 1x bright daylight
- 1x golden hour / warm light
- 1x overcast / flat lighting
- 1x artificial indoor lighting
- 1x dramatic / high contrast (strong shadows)
- 1x low light / night

#### Complexity Diversity (6 images)
- 2x simple (single subject, clean background)
- 2x moderate (subject with props or secondary elements)
- 2x complex (busy scene, multiple elements, text/signs visible)

### Overlap Strategy

Images should cover multiple dimensions simultaneously. For example, a "close-up portrait in golden hour lighting at an outdoor cafe" covers subject type, setting, and lighting in one image. Target 30 images total, not 30 per category.

### Evaluation Procedure (for interactive phase)

1. Run each provider on all 30 images with the default prompt template.
2. Manually score each caption on the 6 criteria (1-5 scale).
3. Calculate weighted scores per provider.
4. Document notable failure cases (hallucinations, refusals, truncations).
5. Compare top 2-3 providers on 5 "challenge" images (low light, complex scene, ambiguous content).

---

## 7. Appendix: Pricing Comparison Table

| Provider | Model | Input $/1M tok | Output $/1M tok | Est. $/caption | Est. cost (500 imgs) |
|----------|-------|----------------|-----------------|----------------|---------------------|
| Anthropic | Claude Sonnet 4 | $3.00 | $15.00 | $0.0074 | $3.70 |
| Anthropic | Claude Opus 4 | $15.00 | $75.00 | $0.0375 | $18.75 |
| Anthropic | Claude Haiku 3.5 | $0.80 | $4.00 | $0.002 | $1.00 |
| OpenAI | GPT-4o | $2.50 | $10.00 | $0.0042 | $2.10 |
| OpenAI | GPT-4o-mini | $0.15 | $0.60 | $0.0003 | $0.15 |
| Google | Gemini 2.5 Flash | $0.15 | $0.60 | $0.0002 | $0.10 |
| Google | Gemini 2.5 Pro | $1.25 | $10.00 | $0.0024 | $1.20 |
| xAI | Grok 2 Vision | $2.00 | $10.00 | $0.004 | $2.00 |
| Local | Any (7B-72B) | $0 | $0 | $0 | $0 (hw cost only) |

**Assumptions**: 1024x1024 image, ~100-token system/user prompt, ~200-token caption output. Image token costs estimated per provider's tiling/resizing algorithm. Verify current pricing before implementation.

### Cost Efficiency Ranking

1. **Local models** — $0 marginal cost
2. **Gemini 2.5 Flash** — $0.10 per 500 images
3. **GPT-4o-mini** — $0.15 per 500 images
4. **Claude Haiku 3.5** — $1.00 per 500 images
5. **Gemini 2.5 Pro** — $1.20 per 500 images
6. **Grok 2 Vision** — $2.00 per 500 images
7. **GPT-4o** — $2.10 per 500 images
8. **Claude Sonnet 4** — $3.70 per 500 images
9. **Claude Opus 4** — $18.75 per 500 images

---

## Appendix: Research Caveats

This document was compiled from provider documentation and community knowledge as of May 2025. The following items should be verified against current provider documentation before implementation:

- **Exact pricing**: Providers change pricing frequently. Check each provider's pricing page.
- **Rate limits**: Tier-based limits change with account age and spend. Verify for the specific API key being used.
- **Model availability**: Models may be renamed, deprecated, or replaced. Check current model lists.
- **Claude 4 model IDs**: Claude 4 Sonnet and Opus were released in May 2025. Exact model ID strings should be confirmed.
- **Gemini 2.5 "thinking" behavior**: The thinking budget feature and its cost implications should be tested empirically.
- **Local model compatibility**: Vision support in llama.cpp and related tools evolves rapidly. Test specific model + mmproj combinations before committing to a provider.

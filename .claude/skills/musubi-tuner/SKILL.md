---
name: musubi-tuner
description: LoRA training and inference with kohya's musubi-tuner for video/image generation models. Use when user mentions kohya, musubi-tuner, or asks about training LoRA, finetuning, generating images/videos with HunyuanVideo, Wan2.1, Wan2.2, FramePack, FLUX.1 Kontext, FLUX.2, Qwen-Image, Z-Image, or Kandinsky 5. Covers dataset setup, pre-caching, training commands, inference, and VRAM optimization. Do NOT use for sd-scripts, ComfyUI, or other training frameworks.
disable-model-invocation: false
allowed-tools: Read, Write, Edit, Glob, Grep, Bash
argument-hint: <model-name>
---

# Musubi Tuner

[kohya-ss/musubi-tuner](https://github.com/kohya-ss/musubi-tuner) provides scripts for training LoRA models with multiple video/image generation architectures. This skill covers the full workflow: dataset preparation, pre-caching, training, and inference.

## Important

- This project is experimental and under active development
- Python 3.10+, PyTorch 2.5.1+ required
- Hardware: 12GB+ VRAM for images, 24GB+ for video, 64GB+ system RAM
- fp8_scaled and fp8_e4m3fn model versions **cannot** be used as base weights for Qwen-Image
- Always pre-cache latents AND text encoder outputs before training
- Video frame counts must be in N*4+1 format (e.g. 5, 9, 13, 17, 21, 25...)
- Use `bf16` mixed precision for all architectures

## Reference Files

Load **only** the references relevant to the user's current model and task.

### Model-Specific References

| Model | Reference File | Use When |
|-------|---------------|----------|
| Qwen-Image | `references/qwen-image.md` | Text-to-image, image editing, layered LoRA with Qwen-Image |
| FLUX.2 | `references/flux2.md` | FLUX.2 dev/klein image generation and editing |
| FLUX.1 Kontext | `references/flux-kontext.md` | FLUX.1 Kontext reference-image generation |
| HunyuanVideo | `references/hunyuan-video.md` | HunyuanVideo T2V, SkyReels V1 |
| HunyuanVideo 1.5 | `references/hunyuan-video-1-5.md` | HunyuanVideo 1.5 T2V and I2V |
| Wan 2.1/2.2 | `references/wan.md` | Wan2.1/2.2 T2V, I2V, T2I, Fun Control |
| FramePack | `references/framepack.md` | FramePack I2V, one-frame inference, Kisekaeichi |
| Kandinsky 5 | `references/kandinsky5.md` | Kandinsky 5 Pro T2V and I2V |
| Z-Image | `references/zimage.md` | Z-Image text-to-image generation |

### Common References

| Topic | Reference File | Use When |
|-------|---------------|----------|
| Dataset Config | `references/dataset-config.md` | Setting up TOML dataset configs, image/video datasets, control images |
| Advanced Training | `references/advanced-training.md` | LoRA+, logging, FP8 optimization, torch.compile, schedulers, LoHa/LoKr, EMA merging, captioning |
| Sampling | `references/sampling.md` | Generating sample images/videos during training |

## Workflow Overview

For any model, the workflow is:

1. **Download models** - DiT, VAE, text encoder(s), and optionally image encoder
2. **Prepare dataset** - Create TOML config with image/video paths and captions
3. **Pre-cache latents** - Run model-specific `*_cache_latents.py` script
4. **Pre-cache text encoder outputs** - Run model-specific `*_cache_text_encoder_outputs.py` script
5. **Train** - Run model-specific `*_train_network.py` via `accelerate launch`
6. **Inference** - Run model-specific `*_generate_*.py` script with trained LoRA

## Installation

```bash
python -m venv venv
source venv/bin/activate  # Linux
pip install torch torchvision --index-url https://download.pytorch.org/whl/cu124
pip install -e .

# Optional
pip install ascii-magic matplotlib tensorboard prompt-toolkit
```

Configure accelerate for single-GPU training:
```bash
accelerate config
# This machine -> No distributed -> NO CPU-only -> NO dynamo -> NO DeepSpeed -> all GPUs -> NO numa -> bf16
```

## Common Memory Optimization Flags

These flags are shared across most architectures:

| Flag | Effect |
|------|--------|
| `--fp8_base` | Run DiT in fp8 (significant VRAM savings) |
| `--fp8_scaled` | Better quality fp8 (use with `--fp8_base`) |
| `--gradient_checkpointing` | Trade compute for VRAM |
| `--gradient_checkpointing_cpu_offload` | Further VRAM reduction |
| `--blocks_to_swap N` | Offload N transformer blocks to CPU |
| `--split_attn` | Process attention in chunks |
| `--sdpa` | PyTorch scaled dot product attention (no extra deps) |

## Instructions

1. **Determine the model architecture.** Check if the user specified a model name (e.g. "qwen-image", "flux2", "wan"). If `$ARGUMENTS` is provided, use it as the model name. If unclear, ask which architecture they are using.
2. **Read the model-specific reference file.** Use the table above to find the correct file under `references/` and read it with the Read tool. This contains the exact script names, required arguments, and recommended settings for that model.
3. **Read common references as needed:**
   - Dataset setup questions -> read `references/dataset-config.md`
   - LoRA+, logging, torch.compile, LoHa/LoKr, EMA merging -> read `references/advanced-training.md`
   - Sample generation during training -> read `references/sampling.md`
4. **Follow the reference file exactly** for script names, `--network_module` values, and `--timestep_sampling` settings. These differ per architecture and using the wrong value will silently produce bad results.
5. **Always verify the user has pre-cached** both latents and text encoder outputs before suggesting a training command. Missing pre-cache is the most common setup error.

## Examples

**Example 1: User wants to train a Qwen-Image LoRA**
- User says: "I want to train a LoRA on qwen-image with my dataset"
- Actions: Read `references/qwen-image.md`. Help with dataset TOML config, pre-cache commands, then training command with `--network_module networks.lora_qwen_image`.

**Example 2: User gets OOM during training**
- User says: "I'm running out of VRAM training HunyuanVideo"
- Actions: Read `references/hunyuan-video.md`. Suggest progressive memory reduction: `--fp8_base` -> `--gradient_checkpointing` -> `--blocks_to_swap N` (up to max for that architecture).

**Example 3: User wants to write a dataset config**
- User says: "How do I set up my images for training?"
- Actions: Read `references/dataset-config.md`. Ask whether they have caption .txt files or want JSONL metadata. Generate appropriate TOML config.

## Troubleshooting

### OOM / CUDA out of memory during training
Add flags in this order until it fits: `--fp8_base --fp8_scaled` -> `--gradient_checkpointing` -> `--blocks_to_swap N` (check model-specific max) -> `--gradient_checkpointing_cpu_offload`. Each reference file lists VRAM estimates.

### Black or corrupted output videos
PyTorch version too old. Requires PyTorch 2.5.1 or later. Run `pip install torch torchvision --index-url https://download.pytorch.org/whl/cu124`.

### Training produces no improvement
1. Verify pre-cache was run (both latents AND text encoder outputs) with the correct model-specific scripts
2. Check that `--network_module` matches the architecture (e.g. `networks.lora_wan` not `networks.lora`)
3. Check `--timestep_sampling` and `--discrete_flow_shift` values match the model reference

### Wrong frame count errors
Video frame counts must be N*4+1 (5, 9, 13, 17, 21, 25...). The scripts auto-truncate but results may be unexpected.

### fp8 model files don't work as base weights
For Qwen-Image: fp8_scaled and fp8_e4m3fn checkpoint versions cannot be used as base DiT weights. Download the full-precision version.

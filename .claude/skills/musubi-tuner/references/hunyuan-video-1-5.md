# HunyuanVideo 1.5

Text-to-video (T2V) and image-to-video (I2V) generation with HunyuanVideo 1.5.

## Model Downloads

**DiT Models:**
- I2V: `transformer/720p_i2v/diffusion_pytorch_model.safetensors` from HuggingFace
- T2V: `transformer/720p_t2v/diffusion_pytorch_model.safetensors` from HuggingFace
- Alternative: ComfyUI repackaged weights (fp16 - unsuitable for bf16 training)

**VAE:** `vae/diffusion_pytorch_model.safetensors`

**Text Encoders:**
- Qwen2.5-VL: `split_files/text_encoders/qwen_2.5_vl_7b.safetensors`
- BYT5: `split_files/text_encoders/byt5_small_glyphxl_fp16.safetensors`

**Image Encoder (I2V only):**
- SigLIP: `split_files/clip_vision/sigclip_vision_patch14_384.safetensors`

## Pre-caching

### Latent Pre-caching

```bash
python src/musubi_tuner/hv_1_5_cache_latents.py \
  --dataset_config path/to/toml \
  --vae path/to/vae_model
```

- `--vae_sample_size`: VAE tiling size (default 128; 256 for better quality; 0 disables)
- `--vae_enable_patch_conv`: Patch-based convolution for memory optimization
- `--i2v`: Enable I2V mode
- `--image_encoder`: Path for caching image features in I2V mode

### Text Encoder Output Pre-caching

```bash
python src/musubi_tuner/hv_1_5_cache_text_encoder_outputs.py \
  --dataset_config path/to/toml \
  --text_encoder path/to/text_encoder \
  --byt5 path/to/byt5 \
  --batch_size 16
```

- `--fp8_vl`: Run Qwen2.5-VL in fp8 for VRAM savings

## Training

Script: `hv_1_5_train_network.py`

### T2V Training

```bash
accelerate launch --num_cpu_threads_per_process 1 --mixed_precision bf16 \
  src/musubi_tuner/hv_1_5_train_network.py \
  --dit path/to/dit_model \
  --vae path/to/vae_model \
  --text_encoder path/to/text_encoder \
  --byt5 path/to/byt5 \
  --dataset_config path/to/toml \
  --task t2v \
  --sdpa --mixed_precision bf16 \
  --timestep_sampling shift --weighting_scheme none --discrete_flow_shift 2.0 \
  --optimizer_type adamw8bit --learning_rate 1e-4 --gradient_checkpointing \
  --max_data_loader_n_workers 2 --persistent_data_loader_workers \
  --network_module networks.lora_hv_1_5 --network_dim 32 \
  --max_train_epochs 16 --save_every_n_epochs 1 --seed 42 \
  --output_dir path/to/output_dir --output_name name-of-lora
```

### I2V Training

Same as T2V but add:
```bash
  --task i2v \
  --image_encoder path/to/image_encoder \
```

### Required Arguments

- `--network_module networks.lora_hv_1_5`
- `--task`: `t2v` or `i2v`
- `--byt5`: BYT5 text encoder path
- I2V requires `--image_encoder`
- Recommended optimizer: `Muon` (PyTorch 2.9+); use `adamw8bit` for earlier versions

### Memory Optimization

- `--fp8_base` and `--fp8_scaled`: Reduce DiT memory (use together)
- `--fp8_vl`: Reduce text encoder memory
- `--vae_sample_size`: Control VAE tiling (default 128)
- `--vae_enable_patch_conv`: Patch convolution optimization
- `--gradient_checkpointing`
- `--gradient_checkpointing_cpu_offload`
- `--blocks_to_swap N`: Max 51 blocks

### Attention Options

- `--sdpa`: PyTorch SDPA (no deps)
- `--flash_attn`: FlashAttention
- `--xformers` with `--split_attn`: xformers
- `--sage_attn`: SageAttention (not yet for training)
- `--split_attn`: Chunk attention for VRAM reduction

## LoRA Format Conversion

Convert to ComfyUI format:

```bash
python src/musubi_tuner/networks/convert_hunyuan_video_1_5_lora_to_comfy.py \
  path/to/hv_1_5_lora.safetensors \
  path/to/output_comfy_lora.safetensors
```

- `--reverse`: Convert from ComfyUI back to HV 1.5 format

## Inference

Script: `hv_1_5_generate_video.py`

Official recommendations: 121 frames, 50 inference steps.

### T2V Inference

```bash
python src/musubi_tuner/hv_1_5_generate_video.py \
  --dit path/to/dit_model \
  --vae path/to/vae_model \
  --text_encoder path/to/text_encoder \
  --byt5 path/to/byt5 \
  --prompt "A cat" \
  --video_size 720 1280 --video_length 21 --infer_steps 25 \
  --attn_mode sdpa --fp8_scaled \
  --save_path path/to/save/dir --output_type video \
  --seed 1234 --lora_multiplier 1.0 --lora_weight path/to/lora.safetensors
```

### I2V Inference

Same as T2V but add:
```bash
  --image_encoder path/to/image_encoder \
  --image_path path/to/image.jpg \
  --attn_mode torch \
```

### Inference Parameters

- `--video_size H W`: Height then width
- `--video_length`: Must be N*4+1
- `--fp8_scaled`: DiT memory reduction
- `--vae_sample_size`: VAE tiling (default 128; 256 better quality; 0 disables)
- `--vae_enable_patch_conv`: Patch convolution
- `--blocks_to_swap N`: Max 51
- `--guidance_scale` (default 6.0): CFG
- `--flow_shift` (default 7.0): Discrete flow shift
- `--lora_weight`, `--lora_multiplier`: LoRA loading
- `--lycoris`: LyCORIS support
- `--save_merged_model`: Save merged DiT (skips inference)

### VRAM

720p (1280x720) with 121 frames requires ~20GB VRAM even with `--blocks_to_swap 51`.

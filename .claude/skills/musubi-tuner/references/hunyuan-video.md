# HunyuanVideo

Text-to-video generation with HunyuanVideo and SkyReels V1 support.

## Model Downloads

### ComfyUI Approach (Recommended)

- **DiT/VAE**: HunyuanVideo models from `hunyuan-video-t2v-720p` directory
- **Text Encoder 1 (LLM)**: `llava_llama3_fp16.safetensors` from Comfy-Org
- **Text Encoder 2 (CLIP)**: `clip_l.safetensors` from Comfy-Org
- **Alternative for fp8**: `mp_rank_00_model_states_fp8.safetensors` (unofficial)

### Official Structure

```
ckpts/
  hunyuan-video-t2v-720p/
    transformers/
    vae/
  text_encoder/
  text_encoder_2/
```

## Pre-caching

### Latent Pre-caching

```bash
python src/musubi_tuner/cache_latents.py \
  --dataset_config path/to/toml \
  --vae path/to/ckpts/hunyuan-video-t2v-720p/vae/pytorch_model.pt \
  --vae_chunk_size 32 --vae_tiling
```

- `--vae_spatial_tile_sample_min_size 128`: Reduce for limited VRAM
- `--disable_cudnn_backend`: For AMD GPUs or slow caching
- `--debug_mode image/console/video`: Debug visualization
- `--keep_cache`: Prevent auto-deletion of unused cache files

### Text Encoder Output Pre-caching

```bash
python src/musubi_tuner/cache_text_encoder_outputs.py \
  --dataset_config path/to/toml \
  --text_encoder1 path/to/text_encoder \
  --text_encoder2 path/to/text_encoder_2 \
  --batch_size 16
```

- `--fp8_llm`: fp8 for LLM text encoder (for <16GB VRAM)

## Training

Script: `hv_train_network.py`

```bash
accelerate launch --num_cpu_threads_per_process 1 --mixed_precision bf16 \
  src/musubi_tuner/hv_train_network.py \
  --dit path/to/ckpts/hunyuan-video-t2v-720p/transformers/mp_rank_00_model_states.pt \
  --dataset_config path/to/toml --sdpa --mixed_precision bf16 --fp8_base \
  --optimizer_type adamw8bit --learning_rate 2e-4 --gradient_checkpointing \
  --max_data_loader_n_workers 2 --persistent_data_loader_workers \
  --network_module networks.lora --network_dim 32 \
  --timestep_sampling shift --discrete_flow_shift 7.0 \
  --max_train_epochs 16 --save_every_n_epochs 1 --seed 42 \
  --output_dir path/to/output_dir --output_name name-of-lora
```

### Required Arguments

- `--network_module networks.lora`
- `--timestep_sampling shift --discrete_flow_shift 7.0`

### Memory Optimization

- `--fp8_base`: fp8 for DiT (~16GB VRAM without it)
- `--blocks_to_swap N`: Offload N blocks to CPU (max 36)
- `--use_pinned_memory_for_block_swap`: Improve swap performance
- `--gradient_checkpointing`
- `--gradient_checkpointing_cpu_offload`

### Attention Options

- `--sdpa`: PyTorch SDPA (recommended, no extra deps)
- `--flash_attn`: FlashAttention
- `--xformers` with `--split_attn`: xformers attention
- `--split_attn`: Chunk attention (slight speed reduction, VRAM savings)

## LoRA Merging

```bash
python src/musubi_tuner/merge_lora.py \
  --dit path/to/dit_model \
  --lora_weight path/to/lora.safetensors \
  --save_merged_model path/to/merged_model.safetensors \
  --device cpu --lora_multiplier 1.0
```

Supports multiple LoRA weights with individual multipliers.

## Inference

Script: `hv_generate_video.py`

```bash
python src/musubi_tuner/hv_generate_video.py --fp8 \
  --video_size 544 960 --video_length 5 --infer_steps 30 \
  --prompt "A cat walks on the grass, realistic style." \
  --save_path path/to/save/dir --output_type both \
  --dit path/to/dit_model \
  --attn_mode sdpa --split_attn \
  --vae path/to/vae_model \
  --vae_chunk_size 32 --vae_spatial_tile_sample_min_size 128 \
  --text_encoder1 path/to/text_encoder \
  --text_encoder2 path/to/text_encoder_2 \
  --seed 1234 --lora_multiplier 1.0 --lora_weight path/to/lora.safetensors
```

### Key Inference Parameters

- `--video_size H W`: Height then width
- `--video_length`: Must be N*4+1 format
- `--fp8`: fp8 mode (reduces memory, may impact quality)
- `--fp8_fast`: Faster fp8 on RTX 40x0 (requires `--fp8`)
- `--blocks_to_swap N`: CPU offload (max 38)
- `--attn_mode`: flash/torch/sageattn/xformers/sdpa
- `--output_type`: both/latent/video/images (use "both" for OOM safety)
- `--flow_shift`: 7.0 for 50 steps, 17.0 for <20 steps
- `--compile`: PyTorch compile (experimental, slow first run)
- `--save_merged_model`: Save merged model without inference

### Video-to-Video

- `--video_path`: Video file or image directory
- `--strength` (0-1.0): Change magnitude (experimental)

### SkyReels V1

For T2V (`skyreels_hunyuan_t2v_bf16.safetensors`):
```bash
--guidance_scale 6.0 --embedded_cfg_scale 1.0 \
--negative_prompt "Aerial view, overexposed, low quality, deformation..." \
--split_uncond
```

For I2V (`skyreels_hunyuan_i2v_bf16.safetensors`):
```bash
--image_path path/to/image.jpg
```

- `--guidance_scale 6.0`: CFG scale
- `--embedded_cfg_scale 1.0`: Embedded guidance
- `--split_uncond`: Split uncond/cond (reduces VRAM)

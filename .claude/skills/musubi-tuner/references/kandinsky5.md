# Kandinsky 5

Text-to-video (T2V) and image-to-video (I2V) generation with Kandinsky 5 Pro models.

## Model Downloads

- **DiT**: Pro `.safetensors` checkpoints from Kandinsky 5.0 Collection (e.g., `kandinsky5pro_t2v_pretrain_5s.safetensors`, `kandinsky5pro_i2v_sft_5s.safetensors`)
- **VAE**: HunyuanVideo 3D VAE `diffusion_pytorch_model.safetensors` from hunyuanvideo-community
- **Text Encoder (Qwen2.5-VL)**: `Qwen/Qwen2.5-VL-7B-Instruct` from HuggingFace
- **Text Encoder (CLIP)**: `openai/clip-vit-large-patch14`

## Available Tasks

| Task | Description |
|------|-------------|
| `k5-pro-t2v-5s-sd` | T2V, 5s, 19B, Pro SFT |
| `k5-pro-t2v-10s-sd` | T2V, 10s, 19B, Pro SFT |
| `k5-pro-i2v-5s-sd` | I2V, 5s, 19B, Pro SFT |

Pretrain checkpoints also available for T2V 5s and 10s tasks.

## Pre-caching

### Text Encoder Output Pre-caching

```bash
python src/musubi_tuner/kandinsky5_cache_text_encoder_outputs.py \
  --dataset_config path/to/toml \
  --text_encoder_qwen Qwen/Qwen2.5-VL-7B-Instruct \
  --text_encoder_clip openai/clip-vit-large-patch14 \
  --batch_size 4
```

### Latent Pre-caching

```bash
python src/musubi_tuner/kandinsky5_cache_latents.py \
  --dataset_config path/to/toml \
  --vae path/to/vae/diffusion_pytorch_model.safetensors
```

For NABLA training, add `--nabla_resize`.

## Training

Script: `kandinsky5_train_network.py`

```bash
accelerate launch --num_cpu_threads_per_process 1 --mixed_precision bf16 \
  src/musubi_tuner/kandinsky5_train_network.py \
  --mixed_precision bf16 \
  --dataset_config path/to/toml \
  --task k5-pro-t2v-5s-sd \
  --dit path/to/kandinsky5pro_t2v_pretrain_5s.safetensors \
  --text_encoder_qwen Qwen/Qwen2.5-VL-7B-Instruct \
  --text_encoder_clip openai/clip-vit-large-patch14 \
  --vae path/to/vae/diffusion_pytorch_model.safetensors \
  --fp8_base --sdpa --gradient_checkpointing \
  --max_data_loader_n_workers 1 --persistent_data_loader_workers \
  --learning_rate 1e-4 \
  --optimizer_type AdamW8Bit \
  --optimizer_args "weight_decay=0.001" "betas=(0.9,0.95)" \
  --max_grad_norm 1.0 \
  --lr_scheduler constant_with_warmup --lr_warmup_steps 100 \
  --network_module networks.lora_kandinsky --network_dim 32 --network_alpha 32 \
  --timestep_sampling shift --discrete_flow_shift 5.0 \
  --output_dir path/to/output --output_name k5_lora \
  --save_every_n_epochs 1 --max_train_epochs 50 \
  --scheduler_scale 10.0
```

### Required Arguments

- `--task`: Model configuration
- `--network_module networks.lora_kandinsky`
- `--text_encoder_qwen` and `--text_encoder_clip`
- `--scheduler_scale`: Override task's scheduler scaling factor

### Memory Optimization

- `--fp8_base`: DiT in fp8
- `--blocks_to_swap N`: Offload to CPU
- `--gradient_checkpointing`
- `--gradient_checkpointing_cpu_offload`
- `--offload_dit_during_sampling`: Offload DiT to CPU during sampling

### Attention Options

`--sdpa`, `--flash_attn`, `--flash3`, `--sage_attn`, `--xformers`

### NABLA Attention Options

- `--force_nabla_attention`: Force NABLA regardless of task
- `--nabla_method`: Binarization method (default: topcdf)
- `--nabla_P`: CDF threshold (default: 0.9)
- `--nabla_wT`, `--nabla_wH`, `--nabla_wW`: STA window sizes (defaults: 11, 3, 3)
- `--nabla_add_sta` / `--no_nabla_add_sta`: Toggle STA prior

## Inference

Script: `kandinsky5_generate_video.py`

```bash
python src/musubi_tuner/kandinsky5_generate_video.py \
  --task k5-pro-t2v-5s-sd \
  --dit path/to/dit_model \
  --vae path/to/vae/diffusion_pytorch_model.safetensors \
  --text_encoder_qwen Qwen/Qwen2.5-VL-7B-Instruct \
  --text_encoder_clip openai/clip-vit-large-patch14 \
  --offload_dit_during_sampling --fp8_base --dtype bfloat16 \
  --prompt "A cat walks on the grass, realistic style." \
  --negative_prompt "low quality, artifacts" \
  --frames 17 --steps 50 --guidance 5 --scheduler_scale 10 \
  --seed 42 --width 512 --height 512 \
  --output path/to/output.mp4 \
  --lora_weight path/to/lora.safetensors --lora_multiplier 1.0
```

### Key Options

- `--frames`: Number of frames
- `--steps`: Inference steps
- `--guidance`: Guidance scale
- `--negative_prompt`: Optional negative prompt
- `--output`: Output file (.mp4 for video, .png for image)
- `--width`, `--height`: Resolution
- `--blocks_to_swap N`: CPU offload

## I2V Notes

Switch task to `k5-pro-i2v-5s-sd` with corresponding checkpoint. The latent cache stores both first and last frame latents, supporting first-only and first+last conditioning without additional flags.

Note: LoHa/LoKr is **not supported** for Kandinsky5.

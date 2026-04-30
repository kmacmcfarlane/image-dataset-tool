# Qwen-Image

Training and inference for Qwen-Image models (text-to-image, image editing, layered segmentation).

## Model Variants

- **Qwen-Image**: Base text-to-image
- **Qwen-Image-Edit**: Image editing (Edit, Edit-2509, Edit-2511)
- **Qwen-Image-Layered**: Layer segmentation LoRA training

## Model Downloads

**DiT**: Download from Comfy-Org repositories. fp8_scaled and fp8_e4m3fn versions **cannot** be used.

**VAE**: VAE encoder from Comfy-Org.

**Text Encoder**: Qwen2.5-VL from Comfy-Org.

## Pre-caching

### Latent Pre-caching

```bash
python src/musubi_tuner/qwen_image_cache_latents.py \
  --dataset_config path/to/toml \
  --vae path/to/vae_model
```

### Text Encoder Output Pre-caching

```bash
python src/musubi_tuner/qwen_image_cache_text_encoder_outputs.py \
  --dataset_config path/to/toml \
  --text_encoder path/to/qwen2.5_vl \
  --batch_size 16
```

- `--fp8_vl`: Run Qwen2.5-VL in fp8 for VRAM savings
- Adjust `--batch_size` per available VRAM

## LoRA Training

Script: `qwen_image_train_network.py`

```bash
accelerate launch --num_cpu_threads_per_process 1 --mixed_precision bf16 \
  src/musubi_tuner/qwen_image_train_network.py \
  --dit path/to/dit_model \
  --vae path/to/vae_model \
  --text_encoder path/to/qwen2.5_vl \
  --dataset_config path/to/toml \
  --sdpa --mixed_precision bf16 \
  --timestep_sampling shift --weighting_scheme none \
  --optimizer_type adamw8bit --learning_rate 1e-4 --gradient_checkpointing \
  --max_data_loader_n_workers 2 --persistent_data_loader_workers \
  --network_module networks.lora_qwen_image --network_dim 32 \
  --max_train_epochs 16 --save_every_n_epochs 1 --seed 42 \
  --output_dir path/to/output_dir --output_name name-of-lora
```

### Required Arguments

- `--network_module networks.lora_qwen_image`
- `--vae` and `--text_encoder`
- `--mixed_precision bf16`

### Memory Optimization

| Config | Approx VRAM |
|--------|-------------|
| Baseline | ~42 GB |
| `--fp8_base --fp8_scaled` | Significant reduction |
| `+ --gradient_checkpointing` | Further reduction |
| `+ --blocks_to_swap` | Down to ~12 GB with aggressive settings |

- `--fp8_base`: fp8 mode for DiT
- `--fp8_scaled`: Better quality fp8 (use with `--fp8_base`)
- `--fp8_vl`: fp8 for text encoder
- `--gradient_checkpointing`: Trade compute for VRAM
- `--blocks_to_swap N`: Offload N blocks to CPU

## Full Finetuning

Script: `qwen_image_train.py`

```bash
accelerate launch --num_cpu_threads_per_process 1 --mixed_precision bf16 \
  src/musubi_tuner/qwen_image_train.py \
  --dit path/to/dit_model \
  --vae path/to/vae_model \
  --text_encoder path/to/qwen2.5_vl \
  --dataset_config path/to/toml \
  --sdpa --mixed_precision bf16 --gradient_checkpointing \
  --optimizer_type adafactor --learning_rate 1e-6 --fused_backward_pass \
  --optimizer_args "relative_step=False" "scale_parameter=False" "warmup_init=False" \
  --max_grad_norm 0 --lr_scheduler constant_with_warmup --lr_warmup_steps 10 \
  --max_data_loader_n_workers 2 --persistent_data_loader_workers \
  --max_train_epochs 16 --save_every_n_epochs 1 --seed 42 \
  --output_dir path/to/output_dir --output_name name-of-model
```

- `--full_bf16`: Load weights in bfloat16 (saves ~30GB VRAM, may impact accuracy)
- Use Adafactor optimizer with `--fused_backward_pass` for limited VRAM
- `--fused_backward_pass` does not support gradient accumulation; use `--max_grad_norm 0`

## Inference

Script: `qwen_image_generate_image.py`

```bash
python src/musubi_tuner/qwen_image_generate_image.py \
  --dit path/to/dit_model \
  --vae path/to/vae_model \
  --text_encoder path/to/qwen2.5_vl \
  --prompt "A cat" \
  --image_size 1024 1024 --infer_steps 25 \
  --attn_mode sdpa \
  --save_path path/to/save/dir --output_type images \
  --seed 1234 --lora_multiplier 1.0 --lora_weight path/to/lora.safetensors
```

### Inference Options

- `--image_size H W`: Height and width
- `--infer_steps`: Number of sampling steps
- `--fp8_scaled`: Reduce DiT memory
- `--fp8_vl`: Reduce text encoder memory
- `--lora_weight`: Path to LoRA weights
- `--lora_multiplier`: LoRA strength (default 1.0)
- `--save_merged_model`: Save DiT after LoRA merge (skips inference)

### Image Editing Mode

For Edit variants, supply control images:

```bash
python src/musubi_tuner/qwen_image_generate_image.py \
  --dit path/to/edit_dit_model \
  --vae path/to/vae_model \
  --text_encoder path/to/qwen2.5_vl \
  --control_image_path path/to/source_image.jpg \
  --prompt "Change the color to red" \
  --image_size 1024 1024 --infer_steps 25 \
  --attn_mode sdpa \
  --save_path path/to/save/dir --output_type images \
  --seed 1234
```

- `--control_image_path`: Source image for editing
- Supports inpainting masks for selective editing
- Reference Consistency Mask (RCM) preserves unmodified regions

### Qwen-Image-Layered

For layered LoRA training, the dataset uses `multiple_target = true` in TOML config. Segmentation layer images are stored alongside base images with numbered suffixes and alpha channels.

During sampling, `--f` specifies output layer count. Total output = `--f` + 1 (original + layers).

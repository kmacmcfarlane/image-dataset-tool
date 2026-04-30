# Advanced Training Configuration

Covers LoRA+, logging, FP8 optimization, torch.compile, LR schedulers, LoHa/LoKr, EMA merging, and captioning tools.

## TOML Configuration Files

Command-line options can be specified in TOML config files for readability:

```toml
[optimizer]
optimizer_type = "adamw8bit"
learning_rate = 1e-4

[training]
max_train_epochs = 16
gradient_checkpointing = true

[output]
output_dir = "path/to/output"
output_name = "my-lora"
```

## LoRA+ (Higher LR for B Matrices)

Improves training speed by applying a different learning rate to LoRA B matrices:

```bash
--network_args "loraplus_lr_ratio=16"
```

The ratio multiplies the base learning rate for the B matrix. Only supported for standard LoRA (not LoHa/LoKr).

## Module Selection

Use regex patterns to include/exclude specific modules:

```bash
--network_args "exclude_patterns=[r'.*norm.*']"
--network_args "include_patterns=[r'.*attn.*']"
```

## Logging

### TensorBoard

```bash
--logging_dir path/to/logs --log_prefix experiment_name
```

### Weights & Biases

```bash
--log_with wandb
```

## FP8 Weight Optimization

Performs offline passes rewriting Linear weights into FP8 (E4M3) with block-wise scaling:

- `--fp8_base`: Run DiT in fp8
- `--fp8_scaled`: Better quality fp8 with per-block scaling (recommended with `--fp8_base`)

## torch.compile

Significant training and inference performance improvements via JIT compilation.

### Prerequisites

- Triton required
- Windows: MSVC compiler for `--compile_dynamic`

### Performance Examples

- Qwen-Image (RTX A6000): ~10.5% faster (default mode)
- RTX PRO 6000 Blackwell: ~25.2% faster (max-autotune-no-cudagraphs)

### Supported Architectures

HunyuanVideo, Wan2.1/2.2, FramePack, FLUX.1 Kontext, Qwen-Image variants.

### Arguments

| Argument | Description |
|----------|-------------|
| `--compile` | Enable optimization |
| `--compile_backend` | Backend (default: `inductor`) |
| `--compile_mode` | `default`, `reduce-overhead`, `max-autotune`, `max-autotune-no-cudagraphs` |
| `--compile_dynamic` | Enable dynamic shapes (`true`/`false`/`auto`) |
| `--compile_fullgraph` | Enable fullgraph mode |
| `--compile_cache_size_limit` | Cache limit (recommended: 32) |
| `--cuda_allow_tf32` | Enable TF32 precision (Ampere+) |
| `--cuda_cudnn_benchmark` | Enable cuDNN benchmark |

### Recommended Settings

**Training:**
```bash
--compile --compile_mode default --compile_cache_size_limit 32 --cuda_allow_tf32 --cuda_cudnn_benchmark
```

**Inference:**
```bash
--compile --compile_mode max-autotune-no-cudagraphs --compile_cache_size_limit 32
```

### Limitations

- `--compile_fullgraph` and `--split_attn` cannot be combined
- `--blocks_to_swap` disables compilation for swapped blocks

## LR Schedulers

Standard schedulers available plus:

- **Rex**: Custom scheduler via `--lr_scheduler rex`
- **Schedule-free optimizers**: e.g., `schedulefree_adamw`

## Timestep Bucketing

Uniform sampling across timestep ranges with `--timestep_bucketing`.

## LoHa / LoKr (LyCORIS)

Alternative parameter-efficient adapters to standard LoRA.

### LoHa (Low-rank Hadamard Product)

```bash
--network_module networks.loha --network_dim 32 --network_alpha 16
```

Roughly 2x trainable parameters vs LoRA at same rank. Captures more complex weight structures.

### LoKr (Low-rank Kronecker Product)

```bash
--network_module networks.lokr --network_dim 32 --network_alpha 16
```

Tends to produce smaller models. Control factorization with `--network_args "factor=4"`.

### Supported Architectures

All except Kandinsky5. Architecture-specific `exclude_patterns` applied automatically.

### Common Options

```bash
--network_args "verbose=True"         # Detailed network info
--network_args "rank_dropout=0.1"     # Rank dimension dropout
--network_args "module_dropout=0.1"   # Module-level dropout
```

### Inference

Built-in support (no extra flags): Wan, FramePack, HunyuanVideo 1.5, FLUX.2, Qwen-Image, Z-Image.

HunyuanVideo and FLUX.1 Kontext require `--lycoris` flag and `pip install lycoris-lora`.

### Limitations

- LoRA+ not supported
- `merge_lora.py` only works with standard LoRA
- `convert_lora.py` supports format conversion for ComfyUI

## LoRA Post-Hoc EMA Merging

Merge multiple checkpoints with exponential moving average:

```bash
python src/musubi_tuner/lora_post_hoc_ema.py \
  lora_epoch_001.safetensors lora_epoch_002.safetensors lora_epoch_003.safetensors \
  --output_file lora_ema_merged.safetensors --beta 0.95
```

### Options

- `--beta`: Decay rate (higher = more weight to average, default 0.95)
- `--beta2`: Second rate for linear interpolation
- `--sigma_rel`: Power Function EMA (mutually exclusive with beta)
- `--no_sort`: Disable auto-sort by modification time

### Recommended Settings (30 epochs)

| Scenario | beta | sigma_rel |
|----------|------|-----------|
| General | 0.9 | 0.2 |
| Early convergence | 0.95 | 0.25 |
| Avoid overfitting | 0.8 | 0.15 |

## Image Captioning with Qwen2.5-VL

```bash
python src/musubi_tuner/caption_images_by_qwen_vl.py \
  --image_dir /path/to/images \
  --model_path /path/to/qwen_model.safetensors \
  --output_file /path/to/captions.jsonl
```

### Options

- `--output_format`: `jsonl` (single file) or `text` (per-image .txt files)
- `--fp8_vl`: Load in fp8 for reduced memory
- `--max_size` (default 1280): Max image size
- `--max_new_tokens` (default 1024): Max tokens per caption
- `--prompt`: Custom prompt (supports `\n`)

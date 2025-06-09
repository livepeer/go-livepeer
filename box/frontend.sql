INSERT INTO public.users (
    id,
    name,
    email,
    created_at,
    last_login,
    is_active,
    provider,
    additional_details
) VALUES (
    'did:privy:cm4n0u53u00cb3gs69juod6xy',
    'PSchroedl',
    NULL,
    '2024-12-13 17:27:48.529405+00',
    NULL,
    'true',
    'github',
    '{}'
);

INSERT INTO public.pipelines (
    id,
    created_at,
    updated_at,
    last_used,
    name,
    description,
    is_private,
    cover_image,
    type,
    sample_code_repo,
    is_featured,
    sample_input_video,
    author,
    model_card,
    key,
    config,
    comfy_ui_json,
    version,
    validation_status,
    prioritized_params
) VALUES (
    'pip_DRQREDnSei4HQyC8',
    '2025-02-05 09:57:34.776507+00',
    '2025-02-05 09:57:34.776507+00',
    NULL,
    'Dreamshaper',
    'A ComfyUI pipeline using Dreamshaper model',
    'false',
    'https://fabowvadozbihurwnvwd.supabase.co/storage/v1/object/public/pipelines//Dreamshaper.png',
    'comfyui',
    NULL,
    'true',
    NULL,
    'did:privy:cm4n0u53u00cb3gs69juod6xy',
    '{}',
    'comfyui',
    '{
        "inputs": {
            "primary": {
                "id": "prompt",
                "type": "textarea",
                "label": "Comfy UI JSON",
                "required": true,
                "fullWidth": true,
                "placeholder": "Enter json object",
                "defaultValue": {
                    "1": {
                        "_meta": { "title": "Load Image" },
                        "inputs": { "image": "example.png" },
                        "class_type": "LoadImage"
                    },
                    "2": {
                        "_meta": { "title": "Depth Anything Tensorrt" },
                        "inputs": {
                            "engine": "depth_anything_vitl14-fp16.engine",
                            "images": ["1", 0]
                        },
                        "class_type": "DepthAnythingTensorrt"
                    },
                    "3": {
                        "_meta": { "title": "TensorRT Loader" },
                        "inputs": {
                            "unet_name": "static-dreamshaper8_SD15_$stat-b-1-h-512-w-512_00001_.engine",
                            "model_type": "SD15"
                        },
                        "class_type": "TensorRTLoader"
                    },
                    "5": {
                        "_meta": { "title": "CLIP Text Encode (Prompt)" },
                        "inputs": { "clip": ["23", 0], "text": "" },
                        "class_type": "CLIPTextEncode"
                    },
                    "6": {
                        "_meta": { "title": "CLIP Text Encode (Prompt)" },
                        "inputs": { "clip": ["23", 0], "text": "" },
                        "class_type": "CLIPTextEncode"
                    },
                    "7": {
                        "_meta": { "title": "KSampler" },
                        "inputs": {
                            "cfg": 1,
                            "seed": 785664736216738,
                            "model": ["24", 0],
                            "steps": 1,
                            "denoise": 1,
                            "negative": ["9", 1],
                            "positive": ["9", 0],
                            "scheduler": "normal",
                            "latent_image": ["16", 0],
                            "sampler_name": "lcm"
                        },
                        "class_type": "KSampler"
                    },
                    "8": {
                        "_meta": { "title": "Load ControlNet Model" },
                        "inputs": {
                            "control_net_name": "control_v11f1p_sd15_depth_fp16.safetensors"
                        },
                        "class_type": "ControlNetLoader"
                    },
                    "9": {
                        "_meta": { "title": "Apply ControlNet" },
                        "inputs": {
                            "image": ["2", 0],
                            "negative": ["6", 0],
                            "positive": ["5", 0],
                            "strength": 1,
                            "control_net": ["10", 0],
                            "end_percent": 1,
                            "start_percent": 0
                        },
                        "class_type": "ControlNetApplyAdvanced"
                    },
                    "10": {
                        "_meta": { "title": "TorchCompileLoadControlNet" },
                        "inputs": {
                            "mode": "reduce-overhead",
                            "backend": "inductor",
                            "fullgraph": false,
                            "controlnet": ["8", 0]
                        },
                        "class_type": "TorchCompileLoadControlNet"
                    },
                    "11": {
                        "_meta": { "title": "Load VAE" },
                        "inputs": { "vae_name": "taesd" },
                        "class_type": "VAELoader"
                    },
                    "13": {
                        "_meta": { "title": "TorchCompileLoadVAE" },
                        "inputs": {
                            "vae": ["11", 0],
                            "mode": "reduce-overhead",
                            "backend": "inductor",
                            "fullgraph": true,
                            "compile_decoder": true,
                            "compile_encoder": true
                        },
                        "class_type": "TorchCompileLoadVAE"
                    },
                    "14": {
                        "_meta": { "title": "VAE Decode" },
                        "inputs": {
                            "vae": ["13", 0],
                            "samples": ["7", 0]
                        },
                        "class_type": "VAEDecode"
                    },
                    "15": {
                        "_meta": { "title": "Preview Image" },
                        "inputs": { "images": ["14", 0] },
                        "class_type": "PreviewImage"
                    },
                    "16": {
                        "_meta": { "title": "Empty Latent Image" },
                        "inputs": {
                            "width": 512,
                            "height": 512,
                            "batch_size": 1
                        },
                        "class_type": "EmptyLatentImage"
                    },
                    "23": {
                        "_meta": { "title": "Load CLIP" },
                        "inputs": {
                            "type": "stable_diffusion",
                            "device": "default",
                            "clip_name": "CLIPText/model.fp16.safetensors"
                        },
                        "class_type": "CLIPLoader"
                    },
                    "24": {
                        "_meta": {
                            "title": "Feature Bank Attention Processor"
                        },
                        "inputs": {
                            "model": ["3", 0],
                            "use_feature_injection": false,
                            "feature_cache_interval": 4,
                            "feature_bank_max_frames": 4,
                            "feature_injection_strength": 0.8,
                            "feature_similarity_threshold": 0.98
                        },
                        "class_type": "FeatureBankAttentionProcessor"
                    }
                }
            },
            "advanced": []
        },
        "version": "0.0.1",
        "metadata": {
            "description": "Configuration for Comfy UI parameters"
        }
    }',
    NULL,
    '1.0.2',
    'pending',
    '[{"name": "Quality", "path": "7/inputs/steps", "field": "text", "nodeId": "7", "widget": "number", "classType": "", "description": "Set a quality level (1-5)", "widgetConfig": {"max": 5, "min": 1, "step": 1, "type": "slider", "default": 1}}, {"name": "Denoise", "path": "7/inputs/denoise", "field": "text", "nodeId": "7", "widget": "number", "classType": "", "description": "Set a denoise level (0.0-1.0)", "widgetConfig": {"max": 1, "min": 0, "step": 0.05, "type": "slider", "default": 1}}, {"name": "Creativity", "path": "9/inputs/strength", "field": "text", "nodeId": "9", "widget": "number", "classType": "", "description": "Set a creativity level (0.0-1.0)", "widgetConfig": {"max": 1, "min": 0.05, "step": 0.05, "type": "slider", "default": 1}}, {"name": "Negative Prompt", "path": "6/inputs/text", "field": "text", "nodeId": "6", "widget": "text", "classType": "", "description": "Set a negative prompt", "widgetConfig": {}}]'
);

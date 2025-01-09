/*
Package worker's module provides multipart form data utilities.

This module extends oapi-codegen Go bindings with multipart writers, enabling request
parameter encoding for inter-node communication (gateway, orchestrator, runner).
*/
package worker

import (
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
)

func NewImageToImageMultipartWriter(w io.Writer, req GenImageToImageMultipartRequestBody) (*multipart.Writer, error) {
	mw := multipart.NewWriter(w)
	writer, err := mw.CreateFormFile("image", req.Image.Filename())
	if err != nil {
		return nil, err
	}
	imageSize := req.Image.FileSize()
	imageRdr, err := req.Image.Reader()
	if err != nil {
		return nil, err
	}
	copied, err := io.Copy(writer, imageRdr)
	if err != nil {
		return nil, err
	}
	if copied != imageSize {
		return nil, fmt.Errorf("failed to copy image to multipart request imageBytes=%v copiedBytes=%v", imageSize, copied)
	}

	if err := mw.WriteField("prompt", req.Prompt); err != nil {
		return nil, err
	}
	if req.ModelId != nil {
		if err := mw.WriteField("model_id", *req.ModelId); err != nil {
			return nil, err
		}
	}
	if req.Loras != nil {
		if err := mw.WriteField("loras", *req.Loras); err != nil {
			return nil, err
		}
	}
	if req.Strength != nil {
		if err := mw.WriteField("strength", fmt.Sprintf("%f", *req.Strength)); err != nil {
			return nil, err
		}
	}
	if req.GuidanceScale != nil {
		if err := mw.WriteField("guidance_scale", fmt.Sprintf("%f", *req.GuidanceScale)); err != nil {
			return nil, err
		}
	}
	if req.ImageGuidanceScale != nil {
		if err := mw.WriteField("image_guidance_scale", fmt.Sprintf("%f", *req.ImageGuidanceScale)); err != nil {
			return nil, err
		}
	}
	if req.NegativePrompt != nil {
		if err := mw.WriteField("negative_prompt", *req.NegativePrompt); err != nil {
			return nil, err
		}
	}
	if req.SafetyCheck != nil {
		if err := mw.WriteField("safety_check", strconv.FormatBool(*req.SafetyCheck)); err != nil {
			return nil, err
		}
	}
	if req.Seed != nil {
		if err := mw.WriteField("seed", strconv.Itoa(*req.Seed)); err != nil {
			return nil, err
		}
	}
	if req.NumImagesPerPrompt != nil {
		if err := mw.WriteField("num_images_per_prompt", strconv.Itoa(*req.NumImagesPerPrompt)); err != nil {
			return nil, err
		}
	}
	if req.NumInferenceSteps != nil {
		if err := mw.WriteField("num_inference_steps", strconv.Itoa(*req.NumInferenceSteps)); err != nil {
			return nil, err
		}
	}

	if err := mw.Close(); err != nil {
		return nil, err
	}

	return mw, nil
}

func NewImageToVideoMultipartWriter(w io.Writer, req GenImageToVideoMultipartRequestBody) (*multipart.Writer, error) {
	mw := multipart.NewWriter(w)
	writer, err := mw.CreateFormFile("image", req.Image.Filename())
	if err != nil {
		return nil, err
	}
	imageSize := req.Image.FileSize()
	imageRdr, err := req.Image.Reader()
	if err != nil {
		return nil, err
	}
	copied, err := io.Copy(writer, imageRdr)
	if err != nil {
		return nil, err
	}
	if copied != imageSize {
		return nil, fmt.Errorf("failed to copy image to multipart request imageBytes=%v copiedBytes=%v", imageSize, copied)
	}

	if req.ModelId != nil {
		if err := mw.WriteField("model_id", *req.ModelId); err != nil {
			return nil, err
		}
	}
	if req.Height != nil {
		if err := mw.WriteField("height", strconv.Itoa(*req.Height)); err != nil {
			return nil, err
		}
	}
	if req.Width != nil {
		if err := mw.WriteField("width", strconv.Itoa(*req.Width)); err != nil {
			return nil, err
		}
	}
	if req.Fps != nil {
		if err := mw.WriteField("fps", strconv.Itoa(*req.Fps)); err != nil {
			return nil, err
		}
	}
	if req.MotionBucketId != nil {
		if err := mw.WriteField("motion_bucket_id", strconv.Itoa(*req.MotionBucketId)); err != nil {
			return nil, err
		}
	}
	if req.NoiseAugStrength != nil {
		if err := mw.WriteField("noise_aug_strength", fmt.Sprintf("%f", *req.NoiseAugStrength)); err != nil {
			return nil, err
		}
	}
	if req.Seed != nil {
		if err := mw.WriteField("seed", strconv.Itoa(*req.Seed)); err != nil {
			return nil, err
		}
	}
	if req.SafetyCheck != nil {
		if err := mw.WriteField("safety_check", strconv.FormatBool(*req.SafetyCheck)); err != nil {
			return nil, err
		}
	}
	if req.NumInferenceSteps != nil {
		if err := mw.WriteField("num_inference_steps", strconv.Itoa(*req.NumInferenceSteps)); err != nil {
			return nil, err
		}
	}

	if err := mw.Close(); err != nil {
		return nil, err
	}

	return mw, nil
}

func NewUpscaleMultipartWriter(w io.Writer, req GenUpscaleMultipartRequestBody) (*multipart.Writer, error) {
	mw := multipart.NewWriter(w)
	writer, err := mw.CreateFormFile("image", req.Image.Filename())
	if err != nil {
		return nil, err
	}
	imageSize := req.Image.FileSize()
	imageRdr, err := req.Image.Reader()
	if err != nil {
		return nil, err
	}
	copied, err := io.Copy(writer, imageRdr)
	if err != nil {
		return nil, err
	}
	if copied != imageSize {
		return nil, fmt.Errorf("failed to copy image to multipart request imageBytes=%v copiedBytes=%v", imageSize, copied)
	}

	if err := mw.WriteField("prompt", req.Prompt); err != nil {
		return nil, err
	}
	if req.ModelId != nil {
		if err := mw.WriteField("model_id", *req.ModelId); err != nil {
			return nil, err
		}
	}
	if req.SafetyCheck != nil {
		if err := mw.WriteField("safety_check", strconv.FormatBool(*req.SafetyCheck)); err != nil {
			return nil, err
		}
	}
	if req.Seed != nil {
		if err := mw.WriteField("seed", strconv.Itoa(*req.Seed)); err != nil {
			return nil, err
		}
	}
	if req.NumInferenceSteps != nil {
		if err := mw.WriteField("num_inference_steps", strconv.Itoa(*req.NumInferenceSteps)); err != nil {
			return nil, err
		}
	}

	if err := mw.Close(); err != nil {
		return nil, err
	}

	return mw, nil
}

func NewAudioToTextMultipartWriter(w io.Writer, req GenAudioToTextMultipartRequestBody) (*multipart.Writer, error) {
	mw := multipart.NewWriter(w)
	writer, err := mw.CreateFormFile("audio", req.Audio.Filename())
	if err != nil {
		return nil, err
	}
	audioSize := req.Audio.FileSize()
	audioRdr, err := req.Audio.Reader()
	if err != nil {
		return nil, err
	}
	copied, err := io.Copy(writer, audioRdr)
	if err != nil {
		return nil, err
	}
	if copied != audioSize {
		return nil, fmt.Errorf("failed to copy audio to multipart request audioBytes=%v copiedBytes=%v", audioSize, copied)
	}

	if req.ModelId != nil {
		if err := mw.WriteField("model_id", *req.ModelId); err != nil {
			return nil, err
		}
	}

	if req.ReturnTimestamps != nil {
		if err := mw.WriteField("return_timestamps", *req.ReturnTimestamps); err != nil {
			return nil, err
		}
	}

	if req.Metadata != nil {
		if err := mw.WriteField("metadata", *req.Metadata); err != nil {
			return nil, err
		}
	}

	if err := mw.Close(); err != nil {
		return nil, err
	}

	return mw, nil
}

func NewLLMMultipartWriter(w io.Writer, req BodyGenLLM) (*multipart.Writer, error) {
	mw := multipart.NewWriter(w)

	if err := mw.WriteField("prompt", req.Prompt); err != nil {
		return nil, fmt.Errorf("failed to write prompt field: %w", err)
	}

	if req.History != nil {
		if err := mw.WriteField("history", *req.History); err != nil {
			return nil, fmt.Errorf("failed to write history field: %w", err)
		}
	}

	if req.ModelId != nil {
		if err := mw.WriteField("model_id", *req.ModelId); err != nil {
			return nil, fmt.Errorf("failed to write model_id field: %w", err)
		}
	}

	if req.SystemMsg != nil {
		if err := mw.WriteField("system_msg", *req.SystemMsg); err != nil {
			return nil, fmt.Errorf("failed to write system_msg field: %w", err)
		}
	}

	if req.Temperature != nil {
		if err := mw.WriteField("temperature", fmt.Sprintf("%f", *req.Temperature)); err != nil {
			return nil, fmt.Errorf("failed to write temperature field: %w", err)
		}
	}

	if req.MaxTokens != nil {
		if err := mw.WriteField("max_tokens", strconv.Itoa(*req.MaxTokens)); err != nil {
			return nil, fmt.Errorf("failed to write max_tokens field: %w", err)
		}
	}

	if req.Stream != nil {
		if err := mw.WriteField("stream", fmt.Sprintf("%v", *req.Stream)); err != nil {
			return nil, fmt.Errorf("failed to write stream field: %w", err)
		}
	}

	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	return mw, nil
}

func NewSegmentAnything2MultipartWriter(w io.Writer, req GenSegmentAnything2MultipartRequestBody) (*multipart.Writer, error) {
	mw := multipart.NewWriter(w)
	writer, err := mw.CreateFormFile("image", req.Image.Filename())
	if err != nil {
		return nil, err
	}
	imageSize := req.Image.FileSize()
	imageRdr, err := req.Image.Reader()
	if err != nil {
		return nil, err
	}
	copied, err := io.Copy(writer, imageRdr)
	if err != nil {
		return nil, err
	}
	if copied != imageSize {
		return nil, fmt.Errorf("failed to copy image to multipart request imageBytes=%v copiedBytes=%v", imageSize, copied)
	}

	// Handle input fields.
	if req.ModelId != nil {
		if err := mw.WriteField("model_id", *req.ModelId); err != nil {
			return nil, err
		}
	}
	if req.PointCoords != nil {
		if err := mw.WriteField("point_coords", *req.PointCoords); err != nil {
			return nil, err
		}
	}
	if req.PointLabels != nil {
		if err := mw.WriteField("point_labels", *req.PointLabels); err != nil {
			return nil, err
		}
	}
	if req.Box != nil {
		if err := mw.WriteField("box", *req.Box); err != nil {
			return nil, err
		}
	}
	if req.MaskInput != nil {
		if err := mw.WriteField("mask_input", *req.MaskInput); err != nil {
			return nil, err
		}
	}
	if req.MultimaskOutput != nil {
		if err := mw.WriteField("multimask_output", strconv.FormatBool(*req.MultimaskOutput)); err != nil {
			return nil, err
		}
	}
	if req.ReturnLogits != nil {
		if err := mw.WriteField("return_logits", strconv.FormatBool(*req.ReturnLogits)); err != nil {
			return nil, err
		}
	}
	if req.NormalizeCoords != nil {
		if err := mw.WriteField("normalize_coords", strconv.FormatBool(*req.NormalizeCoords)); err != nil {
			return nil, err
		}
	}

	if err := mw.Close(); err != nil {
		return nil, err
	}

	return mw, nil
}

func NewImageToTextMultipartWriter(w io.Writer, req GenImageToTextMultipartRequestBody) (*multipart.Writer, error) {
	mw := multipart.NewWriter(w)
	writer, err := mw.CreateFormFile("image", req.Image.Filename())
	if err != nil {
		return nil, err
	}
	imageSize := req.Image.FileSize()
	imageRdr, err := req.Image.Reader()
	if err != nil {
		return nil, err
	}
	copied, err := io.Copy(writer, imageRdr)
	if err != nil {
		return nil, err
	}
	if copied != imageSize {
		return nil, fmt.Errorf("failed to copy image to multipart request imageBytes=%v copiedBytes=%v", imageSize, copied)
	}

	if req.Prompt != nil {
		if err := mw.WriteField("prompt", *req.Prompt); err != nil {
			return nil, err
		}
	}
	if req.ModelId != nil {
		if err := mw.WriteField("model_id", *req.ModelId); err != nil {
			return nil, err
		}
	}

	if err := mw.Close(); err != nil {
		return nil, err
	}

	return mw, nil
}

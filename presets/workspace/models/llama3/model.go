// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package llama3

import (
	"time"

	kaitov1beta1 "github.com/kaito-project/kaito/api/v1beta1"
	"github.com/kaito-project/kaito/pkg/model"
	"github.com/kaito-project/kaito/pkg/utils/plugin"
	"github.com/kaito-project/kaito/pkg/workspace/inference"
)

func init() {
	plugin.KaitoModelRegister.Register(&plugin.Registration{
		Name:     "llama-3.1-8b-instruct",
		Instance: &llama3A,
	})
}

var llama3A llama3_1_8bInstruct

type llama3_1_8bInstruct struct{}

func (*llama3_1_8bInstruct) GetInferenceParameters() *model.PresetParam {
	return &model.PresetParam{
		ModelFamilyName:           "LLaMa3",
		AccessMode:                string(kaitov1beta1.ModelAccessModePrivate),
		DiskStorageRequirement:    "20Gi",
		GPUCountRequirement:       "1",
		TotalGPUMemoryRequirement: "20Gi",
		PerGPUMemoryRequirement:   "20Gi",
		RuntimeParam: model.RuntimeParam{
			VLLM: model.VLLMParam{
				BaseCommand: inference.DefaultVLLMCommand,
				// empty means use the served model name will fall back to the huggingface repo id
				ModelName:      "",
				ModelRunParams: map[string]string{},
			},
		},
		ReadinessTimeout: time.Duration(30) * time.Minute,
		WorldSize:        1,
		// base image with no model weights
		ImageName: "base",
		Tag:       "0.0.1",
	}
}

func (*llama3_1_8bInstruct) GetTuningParameters() *model.PresetParam {
	return nil // Currently doesn't support fine-tuning
}
func (*llama3_1_8bInstruct) GetDownloadParameters() *model.DownloadParam {
	return &model.DownloadParam{
		RepoId: "meta-llama/Llama-3.1-8B-Instruct",
	}
}
func (*llama3_1_8bInstruct) SupportDistributedInference() bool {
	return false
}
func (*llama3_1_8bInstruct) SupportTuning() bool {
	return false
}

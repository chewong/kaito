// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package mistral

import (
	"time"

	kaitov1beta1 "github.com/kaito-project/kaito/api/v1beta1"
	"github.com/kaito-project/kaito/pkg/model"
	"github.com/kaito-project/kaito/pkg/utils/plugin"
	"github.com/kaito-project/kaito/pkg/workspace/inference"
	metadata "github.com/kaito-project/kaito/presets/workspace/models"
)

func init() {
	plugin.KaitoModelRegister.Register(&plugin.Registration{
		Name:     PresetLlama3_1_8bInstructModel,
		Instance: &llama3A,
	})
	plugin.KaitoModelRegister.Register(&plugin.Registration{
		Name:     PresetLlama3_3_70bInstructModel,
		Instance: &llama3B,
	})
}

const (
	PresetLlama3_1_8bInstructModel  = "llama-3.1-8b-instruct"
	PresetLlama3_3_70bInstructModel = "llama-3.3-70b-instruct"
)

var llama3A llama3_1_8bInstruct

type llama3_1_8bInstruct struct{}

func (*llama3_1_8bInstruct) GetInferenceParameters() *model.PresetParam {
	return &model.PresetParam{
		Metadata:                  metadata.MustGet(PresetLlama3_1_8bInstructModel),
		ImageAccessMode:           string(kaitov1beta1.ModelImageAccessModePublic),
		DiskStorageRequirement:    "20Gi",
		GPUCountRequirement:       "1",
		TotalGPUMemoryRequirement: "20Gi",
		PerGPUMemoryRequirement:   "20Gi",
		RuntimeParam: model.RuntimeParam{
			Transformers: model.HuggingfaceTransformersParam{
				TorchRunParams:    inference.DefaultAccelerateParams,
				ModelRunParams:    map[string]string{},
				BaseCommand:       "accelerate launch",
				InferenceMainFile: inference.DefaultTransformersMainFile,
			},
			VLLM: model.VLLMParam{
				BaseCommand:    inference.DefaultVLLMCommand,
				ModelName:      PresetLlama3_1_8bInstructModel,
				ModelRunParams: map[string]string{},
			},
		},
		ReadinessTimeout: time.Duration(30) * time.Minute,
	}

}

func (*llama3_1_8bInstruct) GetTuningParameters() *model.PresetParam {
	return nil
}

func (*llama3_1_8bInstruct) SupportDistributedInference() bool {
	return false
}
func (*llama3_1_8bInstruct) SupportTuning() bool {
	return false
}

var llama3B llama3_3_70bInstruct

type llama3_3_70bInstruct struct{}

func (*llama3_3_70bInstruct) GetInferenceParameters() *model.PresetParam {
	return &model.PresetParam{
		Metadata:                  metadata.MustGet(PresetLlama3_3_70bInstructModel),
		ImageAccessMode:           string(kaitov1beta1.ModelImageAccessModePublic),
		DiskStorageRequirement:    "140Gi",
		GPUCountRequirement:       "2",
		TotalGPUMemoryRequirement: "160Gi",
		PerGPUMemoryRequirement:   "80Gi",
		RuntimeParam: model.RuntimeParam{
			Transformers: model.HuggingfaceTransformersParam{
				TorchRunParams:    inference.DefaultAccelerateParams,
				ModelRunParams:    map[string]string{},
				BaseCommand:       "accelerate launch",
				InferenceMainFile: inference.DefaultTransformersMainFile,
			},
			VLLM: model.VLLMParam{
				RayLeaderCommand: inference.DefaultVLLMRayLeaderCommand,
				RayWorkerCommand: inference.DefaultVLLMRayWorkerCommand,
				BaseCommand:      inference.DefaultVLLMCommand,
				ModelName:        PresetLlama3_3_70bInstructModel,
				ModelRunParams:   map[string]string{},
			},
		},
		ReadinessTimeout: time.Duration(60) * time.Minute, // Increased timeout for larger model download/load
	}
}

func (*llama3_3_70bInstruct) GetTuningParameters() *model.PresetParam {
	return nil
}

func (*llama3_3_70bInstruct) SupportDistributedInference() bool {
	return true
}

func (*llama3_3_70bInstruct) SupportTuning() bool {
	return false
}

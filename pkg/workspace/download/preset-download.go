package download

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kaitov1beta1 "github.com/kaito-project/kaito/api/v1beta1"
	"github.com/kaito-project/kaito/pkg/model"
	"github.com/kaito-project/kaito/pkg/utils"
	"github.com/kaito-project/kaito/pkg/utils/consts"
	"github.com/kaito-project/kaito/pkg/utils/resources"
	"github.com/kaito-project/kaito/pkg/workspace/manifests"
)

const (
	pythonImage            = "python:3.12-alpine"
	huggingFaceHubToken    = "HUGGING_FACE_HUB_TOKEN"
	modelWeightsFolderPath = "/workspace/weights"
	baseDownloadCommand    = "huggingface-cli download %s"
)

var (
	tolerations = []corev1.Toleration{
		{
			Effect:   corev1.TaintEffectNoSchedule,
			Operator: corev1.TolerationOpEqual,
			Key:      consts.GPUString,
		},
		{
			Effect: corev1.TaintEffectNoSchedule,
			Value:  consts.GPUString,
			Key:    consts.SKUString,
		},
	}
)

func CreatePresetDownloadPVC(ctx context.Context, workspaceObj *kaitov1beta1.Workspace, kubeClient client.Client) (*corev1.PersistentVolumeClaim, error) {
	pvcObj := manifests.GeneratePVCManifest(workspaceObj)
	err := resources.CreateResource(ctx, pvcObj, kubeClient)
	if client.IgnoreAlreadyExists(err) != nil {
		return nil, err
	}
	return pvcObj, nil
}

func CreatePresetDownloadJob(ctx context.Context, workspaceObj *kaitov1beta1.Workspace, pvc *corev1.PersistentVolumeClaim, downloadObj *model.DownloadParam, kubeClient client.Client) (*batchv1.Job, error) {
	commands := prepareDownloadParameters(downloadObj)
	volumes := []corev1.Volume{
		{
			Name: "weights",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "weights",
			MountPath: modelWeightsFolderPath,
		},
	}

	jobObj := manifests.GenerateDownloadJob(ctx, workspaceObj, pythonImage, commands, tolerations, volumes, volumeMounts, workspaceObj.Inference.Preset.Env)
	err := resources.CreateResource(ctx, jobObj, kubeClient)
	if client.IgnoreAlreadyExists(err) != nil {
		return nil, err
	}
	return jobObj, nil
}

func prepareDownloadParameters(downloadObj *model.DownloadParam) []string {
	// --include flag doesn't work well with equal sign (--include='*.safetensors' '*.json')
	// so we need to use a workaround by passing the arguments with an empty value to achieve
	// --include '*.safetensors' '*.json'
	downloadParam := map[string]string{
		"local-dir":                        modelWeightsFolderPath,
		"include '*.safetensors' '*.json'": "",
	}
	if downloadObj.Revision != "" {
		downloadParam["revision"] = downloadObj.Revision
	}

	commands := []string{
		// TODO(chewong): pin the version of huggingface-hub[cli]
		"pip install huggingface-hub[cli]==0.30.2",
		utils.BuildCmdStr(fmt.Sprintf(baseDownloadCommand, downloadObj.RepoId), downloadParam),
	}

	// Concatenate the commands before returning
	return utils.ShellCmd(strings.Join(commands, " && "))
}

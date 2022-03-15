package cmbv1

var (
	KeyApplication = "application"

	KeyComponent = "component"

	KeyStorage = "storage"

	KeyConfigMap = "configMap"

	KeySecret = "secret"

	KeyMountPath = "mountPath"

	KeyData = "data"
)

type JSONStruct map[string]interface{}

type StorageItemResponse struct {
	AppPrimaryKey string `json:"appPrimaryKey"`
	ComponentName string `json:"componentName"`
	MountPath     string `json:"mountPath"`
	Key           string `json:"key"`
	Type          string `json:"type"`
	Value         string `json:"value,omitempty"`
}

type StorageItemOptions struct {
	Type      string `json:"type"`
	Name      string `json:"name,omitempty"`
	MountPath string `json:"mountPath"`
	DataKey   string `json:"dataKey"`
}

type StorageItemRequest struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	MountPath   string `json:"mountPath"`
	DataKey     string `json:"dataKey"`
	DataValue   string `json:"dataValue"`
	Alias       string `json:"alias,omitempty"`
	Description string `json:"description,omitempty"`
}

type StorageTraitCM struct {
	Name        string            `json:"name"`
	MountPath   string            `json:"mountPath"`
	Data        map[string]string `json:"data"`
	DefaultMode int               `json:"defaultMode,omitempty"`
	MountOnly   bool              `json:"mountOnly,omitempty"`
}

type MountFileInfo struct {
	MountPath string `json:"mountPath"`
	DataKey   string `json:"dataKey"`
	URL       string `json:"url"`
}
type MountFileTreeNode struct {
	Name          string               `json:"name"`
	ParentPath    string               `json:"parentPath"`
	URL           string               `json:"url"`
	MountPath     string               `json:"mountPath"`
	DataKey       string               `json:"dataKey"`
	ChildrenNodes *[]MountFileTreeNode `json:"childrenNodes"`
	IsFile        bool                 `json:"isFile"`
}

type MountFileTreeResponse struct {
	NodeTree *[]MountFileTreeNode `json:"nodeTree"`
}

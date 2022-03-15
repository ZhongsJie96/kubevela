package usecase

import (
	"context"
	"encoding/json"
	mapset "github.com/deckarep/golang-set"
	"github.com/oam-dev/kubevela/pkg/apiserver/clients"
	"github.com/oam-dev/kubevela/pkg/apiserver/datastore"
	"github.com/oam-dev/kubevela/pkg/apiserver/log"
	"github.com/oam-dev/kubevela/pkg/apiserver/model"
	apisCmb "github.com/oam-dev/kubevela/pkg/apiserver/rest/apis/cmbv1"
	"github.com/oam-dev/kubevela/pkg/apiserver/rest/utils/bcode"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

type traitUsecaseImpl struct {
	ds         datastore.DataStore
	kubeClient client.Client
}

type TraitUsecase interface {
	DetailComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, itemOptions *apisCmb.StorageItemOptions) (*apisCmb.StorageItemResponse, error)
	DeleteComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, itemOptions *apisCmb.StorageItemOptions) error
	UpdateComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, updateReq apisCmb.StorageItemRequest) (*model.ApplicationTrait, error)
	CreateComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, creatReq apisCmb.StorageItemRequest) (*model.ApplicationTrait, error)
	GetComponentStorageCMTree(ctx context.Context, app *model.Application, component *model.ApplicationComponent) (*apisCmb.MountFileTreeResponse, error)
}

// NewTraitUsecase new trait usecase 新建一个trait case
func NewTraitUsecase(ds datastore.DataStore) TraitUsecase {
	kubeClient, err := clients.GetKubeClient()
	if err != nil {
		log.Logger.Fatalf("get kubeclient failure %s", err.Error())
	}
	return &traitUsecaseImpl{
		ds:         ds,
		kubeClient: kubeClient,
	}
}

func (t *traitUsecaseImpl) GetComponentStorageCMTree(ctx context.Context, app *model.Application, component *model.ApplicationComponent) (*apisCmb.MountFileTreeResponse, error) {
	var comp = model.ApplicationComponent{
		AppPrimaryKey: app.PrimaryKey(),
		Name:          component.Name,
	}
	if err := t.ds.Get(ctx, &comp); err != nil {
		return nil, err
	}
	// 遍历
	for _, trait := range comp.Traits {
		// 获取storage Trait信息
		if strings.Compare(trait.Type, apisCmb.KeyStorage) == 0 {
			fileInfos, err := GetConfigMapFileInfos(trait.Properties)
			if err != nil {
				return nil, err
			}
			fileTree, err := GetConfigMapFileTree(fileInfos)
			if err != nil {
				return nil, err
			}
			return &apisCmb.MountFileTreeResponse{
				NodeTree: &fileTree,
			}, nil

		}
	}
	return nil, nil
}

func GetConfigMapFileTree(fileInfos []apisCmb.MountFileInfo) ([]apisCmb.MountFileTreeNode, error) {
	var fileTreeNodes []apisCmb.MountFileTreeNode
	if len(fileInfos) > 0 {
		nodeSet := mapset.NewSet()
		for _, fileInfo := range fileInfos {
			SplitPath(nodeSet, fileInfo)
		}

		for _, node := range nodeSet.ToSlice() {
			treeNode, ok := node.(apisCmb.MountFileTreeNode)
			if !ok {
				return nil, bcode.ErrTypeAssert
			}
			if treeNode.ParentPath == "" {
				fileTreeNodes = append(fileTreeNodes, generateChildNode(treeNode, nodeSet))
			}
		}
	}
	return fileTreeNodes, nil
}

func generateChildNode(node apisCmb.MountFileTreeNode, nodeSet mapset.Set) apisCmb.MountFileTreeNode {
	if node.IsFile {
		return node
	}
	slice := nodeSet.ToSlice()
	for _, nodeItem := range slice {
		treeNode := nodeItem.(apisCmb.MountFileTreeNode)
		if treeNode.ParentPath == node.URL {
			if node.ChildrenNodes == nil {
				node.ChildrenNodes = &[]apisCmb.MountFileTreeNode{}
			}
			*node.ChildrenNodes = append(*node.ChildrenNodes, generateChildNode(treeNode, nodeSet))
		}

	}
	return node
}

func SplitPath(nodeSet mapset.Set, fileInfo apisCmb.MountFileInfo) {
	linuxPathReg := "^\\/(\\w+\\/?)+$"
	// linux
	if isLinuxPath, _ := regexp.MatchString(linuxPathReg, fileInfo.MountPath); isLinuxPath {
		split := strings.Split(fileInfo.URL, "/")
		url := ""
		for idx, itemName := range split {
			if idx != 0 && itemName == "" {
				continue
			}
			node := apisCmb.MountFileTreeNode{
				ParentPath: url,
				Name:       itemName,
				IsFile:     false,
			}
			if idx == 0 {
				node.Name = "/" + itemName
			} else {
				node.Name = itemName
			}
			if strings.HasSuffix(url, "/") {
				url += itemName
			} else {
				url += "/" + itemName
			}
			node.URL = url
			if idx == len(split)-1 {
				node.DataKey = fileInfo.DataKey
				node.MountPath = fileInfo.MountPath
				node.IsFile = true
			}
			nodeSet.Add(node)
		}
	} else {
		//	windows
		split := strings.Split(fileInfo.URL, "\\")
		url := ""
		for idx, itemName := range split {
			node := apisCmb.MountFileTreeNode{
				ParentPath: url,
				Name:       itemName,
				IsFile:     false,
			}
			if idx == 0 {
				url += itemName
			} else {
				url += "\\" + itemName
			}
			node.URL = url
			if idx == len(split)-1 {
				node.DataKey = fileInfo.DataKey
				node.MountPath = fileInfo.MountPath
				node.IsFile = true
			}
			nodeSet.Add(node)
		}
	}
}

func GetConfigMapFileInfos(prop *model.JSONStruct) ([]apisCmb.MountFileInfo, error) {
	properties := prop
	propJSON, err := json.Marshal(properties)
	if err != nil {
		return nil, err
	}
	cmSliceRes := gjson.Get(string(propJSON), apisCmb.KeyConfigMap)
	var storageTraitCMS []apisCmb.StorageTraitCM
	err = json.Unmarshal([]byte(cmSliceRes.Raw), &storageTraitCMS)
	if err != nil {
		return nil, err
	}
	var mountFileInfos []apisCmb.MountFileInfo
	for _, traitCM := range storageTraitCMS {
		path := traitCM.MountPath
		linuxPathReg := "^\\/(\\w+\\/?)+$"
		isLinuxPath, _ := regexp.MatchString(linuxPathReg, path)

		if isLinuxPath {
			if !strings.HasSuffix(path, "/") {
				path += "/"
			}
		} else {
			if !strings.HasSuffix(path, "\\") {
				path += "\\"
			}
		}
		for key := range traitCM.Data {
			mountFileInfos = append(mountFileInfos, apisCmb.MountFileInfo{
				MountPath: traitCM.MountPath,
				DataKey:   key,
				URL:       path + key,
			})
		}
	}
	return mountFileInfos, nil
}

func (t *traitUsecaseImpl) CreateComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, createReq apisCmb.StorageItemRequest) (*model.ApplicationTrait, error) {
	var comp = model.ApplicationComponent{
		AppPrimaryKey: app.PrimaryKey(),
		Name:          component.Name,
	}
	if err := t.ds.Get(ctx, &comp); err != nil {
		return nil, err
	}

	if createReq.Name == "" {
		// 用于资源命名
		createReq.Name = component.Name + strings.ReplaceAll(createReq.MountPath, "/", "-") + createReq.DataKey
	}
	var hasStorageTrait bool
	var storageTrait *model.ApplicationTrait
	// 1. 确定是否有Storage Trait
	for _, trait := range comp.Traits {
		if strings.Compare(trait.Type, apisCmb.KeyStorage) == 0 {
			hasStorageTrait = true
			storageTrait = &trait
		}
	}
	if hasStorageTrait {
		// 2. 补充
		if err := mergeStorageItem(storageTrait, createReq); err != nil {
			return nil, err
		}
	} else {
		// 3. 新增
		storageTrait, err := creatStorageTraitAndItem(createReq)
		if err != nil {
			return nil, err
		}
		comp.Traits = append(comp.Traits, *storageTrait)
	}
	if err := t.ds.Put(ctx, &comp); err != nil {
		return nil, err
	}
	return &model.ApplicationTrait{Type: apisCmb.KeyStorage, Properties: storageTrait.Properties, CreateTime: time.Now()}, nil
}

func mergeStorageItem(storageTrait *model.ApplicationTrait, createReq apisCmb.StorageItemRequest) error {
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Info("mergeStorageItem ----> recover: %s", r)
		}
	}()
	storageProperties := *storageTrait.Properties
	if typeProperties, ok := storageProperties[createReq.Type]; ok {
		// 1. 存在特定类型storage
		if _, ok := typeProperties.([]interface{}); !ok {
			return bcode.ErrTypeAssert
		}
		for _, item := range typeProperties.([]interface{}) {
			if item.(map[string]interface{})[apisCmb.KeyMountPath] == createReq.MountPath {
				// 1.1. 存在相同挂载路径cm，只需要添加数据
				itemDataTemp, ok := item.(map[string]interface{})[apisCmb.KeyData]
				if !ok {
					break
				}
				if itemDataMap, ok := itemDataTemp.(map[string]interface{}); ok {
					if _, ok := itemDataMap[createReq.DataKey]; ok {
						return bcode.ErrStorageTraitKeyIsExists
					}
					itemDataMap[createReq.DataKey] = createReq.DataValue
				}
				return nil
			}
		}
		// 1.2. 不存在相同挂载路径cm，新建cm
		dataTemp := make(map[string]string)
		dataTemp[createReq.DataKey] = createReq.DataValue
		storageTraitCMSpec := &apisCmb.StorageTraitCM{
			Name:      createReq.Name,
			MountPath: createReq.MountPath,
			Data:      dataTemp,
		}
		typeProperties := append(typeProperties.([]interface{}), storageTraitCMSpec)
		storageProperties[createReq.Type] = typeProperties
	} else {
		// 2. 不存在特定类型storage
		if createReq.Type == apisCmb.KeyConfigMap {
			dataTemp := make(map[string]string)
			dataTemp[createReq.DataKey] = createReq.DataValue
			storageTraitCMSpec := &apisCmb.StorageTraitCM{
				Name:      createReq.Name,
				MountPath: createReq.MountPath,
				Data:      dataTemp,
			}
			properties, err := model.NewJSONStructByStruct(storageTraitCMSpec)
			if err != nil {
				return bcode.ErrInvalidProperties
			}
			storageProperties[createReq.Type] = properties
		}
	}
	return nil
}

func creatStorageTraitAndItem(createReq apisCmb.StorageItemRequest) (*model.ApplicationTrait, error) {
	dataTemp := make(map[string]string)
	if createReq.Type == apisCmb.KeyConfigMap {
		dataTemp[createReq.DataKey] = createReq.DataValue
		storageTraitCMSpec := &apisCmb.StorageTraitCM{
			Name:      createReq.Name,
			MountPath: createReq.MountPath,
			Data:      dataTemp,
		}
		traitSpec := make(map[string]interface{})
		temp := make([]interface{}, 0, 1)
		traitSpec[createReq.Type] = append(temp, storageTraitCMSpec)
		properties, err := model.NewJSONStructByStruct(traitSpec)
		if err != nil {
			return nil, bcode.ErrInvalidProperties
		}
		return &model.ApplicationTrait{Type: apisCmb.KeyStorage, Properties: properties, CreateTime: time.Now()}, nil
	}

	return nil, bcode.ErrStorageTraitTypeNotSupport
}

func (t *traitUsecaseImpl) UpdateComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, updateReq apisCmb.StorageItemRequest) (*model.ApplicationTrait, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Info("UpdateComponentStorageItem ----> recover: %s", r)
		}
	}()
	var comp = model.ApplicationComponent{
		AppPrimaryKey: app.PrimaryKey(),
		Name:          component.Name,
	}
	if err := t.ds.Get(ctx, &comp); err != nil {
		return nil, err
	}
	for _, trait := range comp.Traits {
		if strings.Compare(trait.Type, apisCmb.KeyStorage) == 0 {
			properties := *trait.Properties
			typeProperties, ok := properties[updateReq.Type]
			if !ok {
				return nil, bcode.ErrStorageTraitTypeNotExists
			}
			for _, item := range typeProperties.([]interface{}) {
				mountPath, ok := item.(map[string]interface{})[apisCmb.KeyMountPath]
				if !ok {
					return nil, bcode.ErrStorageMountPathNotExists
				}
				if mountPath == updateReq.MountPath {
					itemDataTemp, ok := item.(map[string]interface{})[apisCmb.KeyData]
					if !ok {
						return nil, bcode.ErrStorageDataNotExists
					}
					itemDataMap := itemDataTemp.(map[string]interface{})
					if _, ok := itemDataMap[updateReq.DataKey]; ok {
						itemDataMap[updateReq.DataKey] = updateReq.DataValue
						trait.UpdateTime = time.Now()
						if err := t.ds.Put(ctx, &comp); err != nil {
							return nil, err
						}
						return &model.ApplicationTrait{
							Type:        trait.Type,
							Properties:  trait.Properties,
							Alias:       trait.Alias,
							Description: trait.Description,
							CreateTime:  trait.CreateTime,
							UpdateTime:  trait.UpdateTime}, nil
					}
					return nil, bcode.ErrStorageDataNotExists
				}
			}
			return nil, bcode.ErrStorageMountPathNotExists
		}
	}
	return nil, bcode.ErrStorageTraitNotExists
}

func (t *traitUsecaseImpl) DeleteComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, itemOptions *apisCmb.StorageItemOptions) error {
	var comp = model.ApplicationComponent{
		AppPrimaryKey: app.PrimaryKey(),
		Name:          component.Name,
	}
	if err := t.ds.Get(ctx, &comp); err != nil {
		return err
	}
	for _, trait := range comp.Traits {
		if strings.Compare(trait.Type, apisCmb.KeyStorage) == 0 {
			properties := *trait.Properties
			// 1. 找到对应类型的属性配置
			propJSON, err := json.Marshal(properties)
			if err != nil {
				return err

			}
			jsonFieldPath := apisCmb.KeyConfigMap + ".#(" + apisCmb.KeyMountPath +
				"==\"" + itemOptions.MountPath + "\").data"
			dataSliceRes := gjson.Get(string(propJSON), jsonFieldPath)
			if !dataSliceRes.Exists() {
				return bcode.ErrStorageDataNotExists
			}
			switch dataSliceRes.Type {
			case gjson.JSON:
				dataMap := make(map[string]string)
				err := json.Unmarshal([]byte(dataSliceRes.Raw), &dataMap)
				if err != nil {
					return err
				}
				delete(dataMap, itemOptions.DataKey)
				dataMapJSON, err := json.Marshal(dataMap)
				if err != nil {
					return err
				}
				value, _ := sjson.Set(string(propJSON), jsonFieldPath, dataMapJSON)
				var data *model.JSONStruct
				if err := json.Unmarshal([]byte(value), &data); err != nil {
					return err
				}
				trait.Properties = data
				trait.UpdateTime = time.Now()
				if err := t.ds.Put(ctx, &comp); err != nil {
					return err
				}
				return nil
			default:
				return bcode.ErrStorageDataType
			}
		}
	}
	return bcode.ErrStorageTraitNotExists

}

func (t *traitUsecaseImpl) DetailComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, itemOptions *apisCmb.StorageItemOptions) (*apisCmb.StorageItemResponse, error) {
	var comp = model.ApplicationComponent{
		AppPrimaryKey: app.PrimaryKey(),
		Name:          component.Name,
	}
	if err := t.ds.Get(ctx, &comp); err != nil {
		return nil, err
	}
	for _, trait := range comp.Traits {
		if strings.Compare(trait.Type, apisCmb.KeyStorage) == 0 {
			properties := *trait.Properties
			// 1. 找到对应类型的属性配置
			propJSON, err := json.Marshal(properties)
			if err != nil {
				return nil, err

			}
			// gjson key中带有.号处理
			mountPathParam := strings.ReplaceAll(itemOptions.MountPath, ".", "\\.")
			dataKeyParam := strings.ReplaceAll(itemOptions.DataKey, ".", "\\.")
			jsonFieldPath := apisCmb.KeyConfigMap + ".#(" + apisCmb.KeyMountPath +
				"==\"" + mountPathParam + "\").data." + dataKeyParam
			dataRes := gjson.Get(string(propJSON), jsonFieldPath)
			if !dataRes.Exists() {
				return nil, bcode.ErrStorageDataNotExists
			}
			switch dataRes.Type {
			case gjson.String:
				return &apisCmb.StorageItemResponse{
					Type:          itemOptions.Type,
					ComponentName: comp.Name,
					AppPrimaryKey: app.PrimaryKey(),
					MountPath:     itemOptions.MountPath,
					Key:           itemOptions.DataKey,
					Value:         dataRes.Str,
				}, nil
			default:
				return nil, bcode.ErrStorageDataType
			}
		}
	}
	return nil, bcode.ErrStorageTraitNotExists
}

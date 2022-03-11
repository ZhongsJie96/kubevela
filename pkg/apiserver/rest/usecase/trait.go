package usecase

import (
	"context"
	"github.com/oam-dev/kubevela/pkg/apiserver/clients"
	"github.com/oam-dev/kubevela/pkg/apiserver/datastore"
	"github.com/oam-dev/kubevela/pkg/apiserver/log"
	"github.com/oam-dev/kubevela/pkg/apiserver/model"
	apisCmb "github.com/oam-dev/kubevela/pkg/apiserver/rest/apis/cmbv1"
	"github.com/oam-dev/kubevela/pkg/apiserver/rest/utils/bcode"
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
	DeleteComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, itemOptions *apisCmb.StorageItemOptions) (*model.ApplicationTrait, error)
	UpdateComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, updateReq apisCmb.StorageItemRequest) (*model.ApplicationTrait, error)
	CreateComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, creatReq apisCmb.StorageItemRequest) (*model.ApplicationTrait, error)
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

func (t *traitUsecaseImpl) DeleteComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, itemOptions *apisCmb.StorageItemOptions) (*model.ApplicationTrait, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Info("DeleteComponentStorageItem ----> recover: %s", r)
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
			typeProperties, ok := properties[itemOptions.Type]
			if !ok {
				return nil, bcode.ErrStorageTraitTypeNotExists
			}
			for idx, item := range typeProperties.([]interface{}) {
				mountPath, ok := item.(map[string]interface{})[apisCmb.KeyMountPath]
				if !ok {
					return nil, bcode.ErrStorageMountPathNotExists
				}
				if mountPath == itemOptions.MountPath {
					itemDataTemp, ok := item.(map[string]interface{})[apisCmb.KeyData]
					if !ok {
						return nil, bcode.ErrStorageDataNotExists
					}
					itemDataMap := itemDataTemp.(map[string]interface{})
					if _, ok := itemDataMap[itemOptions.DataKey]; ok {
						delete(itemDataMap, itemOptions.DataKey)
						if len(itemDataMap) == 0 {
							typeProperties = append(typeProperties.([]interface{})[:idx], typeProperties.([]interface{})[idx+1:])
						}
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

func (t *traitUsecaseImpl) DetailComponentStorageItem(ctx context.Context, app *model.Application, component *model.ApplicationComponent, itemOptions *apisCmb.StorageItemOptions) (*apisCmb.StorageItemResponse, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Info("DetailComponentStorageItem ----> recover: %s", r)
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
			// 1. 找到对应类型的属性配置
			typeProperties, ok := properties[itemOptions.Type]
			if !ok {
				return nil, bcode.ErrStorageTraitTypeNotExists
			}
			// 2. 根据mountPath查找属性中具体值内容
			var itemData map[string]interface{}
			for _, item := range typeProperties.([]interface{}) {
				// 2.1 对比mountPath确定同一个资源
				mountPath, ok := item.(map[string]interface{})[apisCmb.KeyMountPath]
				if !ok {
					return nil, bcode.ErrStorageMountPathNotExists
				}
				if mountPath == itemOptions.MountPath {
					itemDataTemp, ok := item.(map[string]interface{})[apisCmb.KeyData]
					if !ok {
						return nil, bcode.ErrStorageDataNotExists
					}
					itemData = itemDataTemp.(map[string]interface{})
				}
			}
			if itemData == nil {
				return nil, bcode.ErrStorageMountPathNotExists
			}
			// 3. 根据Key查找到确定条目

			if value, ok := itemData[itemOptions.DataKey]; ok {
				return &apisCmb.StorageItemResponse{
					Type:          itemOptions.Type,
					ComponentName: comp.Name,
					AppPrimaryKey: app.PrimaryKey(),
					MountPath:     itemOptions.MountPath,
					Key:           itemOptions.DataKey,
					Value:         value.(string),
				}, nil
			}
			return nil, bcode.ErrStorageDataNotExists
		}
	}
	return nil, bcode.ErrStorageTraitNotExists
}

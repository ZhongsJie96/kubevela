package webservice

import (
	"context"
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/oam-dev/kubevela/pkg/apiserver/model"
	apisCmb "github.com/oam-dev/kubevela/pkg/apiserver/rest/apis/cmbv1"
	apis "github.com/oam-dev/kubevela/pkg/apiserver/rest/apis/v1"
	"github.com/oam-dev/kubevela/pkg/apiserver/rest/usecase"
	"github.com/oam-dev/kubevela/pkg/apiserver/rest/utils/bcode"
)

type traitService struct {
	traitUsecase       usecase.TraitUsecase
	applicationUsecase usecase.ApplicationUsecase
}

// NewTraitService 将该服务注册大搜webservice中
func NewTraitService(traitUsecase usecase.TraitUsecase, applicationUsecase usecase.ApplicationUsecase) WebService {
	return &traitService{
		traitUsecase:       traitUsecase,
		applicationUsecase: applicationUsecase,
	}

}

func (t *traitService) GetWebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path(versionPrefix+"/traits/applications/{appName}/components/{compName}").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML).
		Doc("api for traits manage")

	tags := []string{"traits"}

	ws.Route(ws.GET("/storage/item").To(t.detailComponentStorageItem).
		Filter(t.appCheckFilter).
		Doc("获取storage trait 条目详情").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("appName", "应用名").DataType("string")).
		Param(ws.PathParameter("compName", "组件名").DataType("string")).
		Param(ws.QueryParameter("type", "storage类型").DataType("string")).
		Param(ws.QueryParameter("mountPath", "挂载路径").DataType("string")).
		Param(ws.QueryParameter("dataKey", "Key").DataType("string")).
		Returns(200, "", apisCmb.StorageItemResponse{}).
		Returns(400, "", bcode.Bcode{}).
		Writes(apisCmb.StorageItemResponse{}))

	ws.Route(ws.DELETE("/storage/item").To(t.deleteComponentStorageItem).
		Filter(t.appCheckFilter).
		Doc("删除storage trait 单条记录").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("appName", "应用名").DataType("string")).
		Param(ws.PathParameter("compName", "组件名").DataType("string")).
		Param(ws.QueryParameter("type", "storage类型").DataType("string")).
		Param(ws.QueryParameter("mountPath", "挂载路径").DataType("string")).
		Param(ws.QueryParameter("dataKey", "Key").DataType("string")).
		Returns(200, "", model.ApplicationTrait{}).
		Returns(400, "", bcode.Bcode{}).
		Writes(model.ApplicationTrait{}))

	ws.Route(ws.PUT("/storage/item").To(t.updateComponentStorageItem).
		Filter(t.appCheckFilter).
		Doc("更新storage trait 单条记录").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("appName", "应用名").DataType("string")).
		Param(ws.PathParameter("compName", "组件名").DataType("string")).
		Reads(apisCmb.StorageItemRequest{}).
		Returns(200, "", model.ApplicationTrait{}).
		Returns(400, "", bcode.Bcode{}).
		Writes(model.ApplicationTrait{}))

	ws.Route(ws.POST("/storage/item").To(t.createComponentStorageItem).
		Filter(t.appCheckFilter).
		Doc("创建storage trait 单条记录").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("appName", "应用名").DataType("string")).
		Param(ws.PathParameter("compName", "组件名").DataType("string")).
		Reads(apisCmb.StorageItemRequest{}).
		Returns(200, "", model.ApplicationTrait{}).
		Returns(400, "", bcode.Bcode{}).
		Writes(model.ApplicationTrait{}))

	return ws
}

func (t *traitService) detailComponentStorageItem(req *restful.Request, res *restful.Response) {
	app := req.Request.Context().Value(&apis.CtxKeyApplication).(*model.Application)
	traitDetail, err := t.traitUsecase.DetailComponentStorageItem(req.Request.Context(), app,
		&model.ApplicationComponent{Name: req.PathParameter("compName")},
		&apisCmb.StorageItemOptions{
			Type:      req.QueryParameter("type"),
			Name:      req.QueryParameter("name"),
			MountPath: req.QueryParameter("mountPath"),
			DataKey:   req.QueryParameter("dataKey"),
		})
	if err != nil {
		bcode.ReturnError(req, res, err)
	}
	if err := res.WriteEntity(traitDetail); err != nil {
		bcode.ReturnError(req, res, err)
		return
	}
}

func (t *traitService) appCheckFilter(req *restful.Request, res *restful.Response, chain *restful.FilterChain) {
	app, err := t.applicationUsecase.GetApplication(req.Request.Context(), req.PathParameter("appName"))
	if err != nil {
		bcode.ReturnError(req, res, err)
		return
	}
	req.Request = req.Request.WithContext(context.WithValue(req.Request.Context(), &apis.CtxKeyApplication, app))
	chain.ProcessFilter(req, res)
}

func (t *traitService) componentCheckFilter(req *restful.Request, res *restful.Response, chain *restful.FilterChain) {
	app := req.Request.Context().Value(&apis.CtxKeyApplication).(*model.Application)
	component, err := t.applicationUsecase.GetApplicationComponent(req.Request.Context(), app, req.PathParameter("compName"))
	if err != nil {
		bcode.ReturnError(req, res, err)
		return
	}
	req.Request = req.Request.WithContext(context.WithValue(req.Request.Context(), &apis.CtxKeyApplicationComponent, component))
	chain.ProcessFilter(req, res)
}

func (t *traitService) deleteComponentStorageItem(req *restful.Request, res *restful.Response) {
	app := req.Request.Context().Value(&apis.CtxKeyApplication).(*model.Application)
	traitDetail, err := t.traitUsecase.DeleteComponentStorageItem(req.Request.Context(), app,
		&model.ApplicationComponent{Name: req.PathParameter("compName")},
		&apisCmb.StorageItemOptions{
			Type:      req.QueryParameter("type"),
			Name:      req.QueryParameter("name"),
			MountPath: req.QueryParameter("mountPath"),
			DataKey:   req.QueryParameter("dataKey"),
		})
	if err != nil {
		bcode.ReturnError(req, res, err)
	}
	if err := res.WriteEntity(traitDetail); err != nil {
		bcode.ReturnError(req, res, err)
		return
	}
}

func (t *traitService) updateComponentStorageItem(req *restful.Request, res *restful.Response) {
	app := req.Request.Context().Value(&apis.CtxKeyApplication).(*model.Application)
	var updateReq apisCmb.StorageItemRequest
	if err := req.ReadEntity(&updateReq); err != nil {
		bcode.ReturnError(req, res, err)
		return
	}

	if err := validate.Struct(&updateReq); err != nil {
		bcode.ReturnError(req, res, err)
		return
	}

	trait, err := t.traitUsecase.UpdateComponentStorageItem(req.Request.Context(), app,
		&model.ApplicationComponent{Name: req.PathParameter("compName")}, updateReq)

	if err != nil {
		bcode.ReturnError(req, res, err)
		return
	}
	if err := res.WriteEntity(trait); err != nil {
		bcode.ReturnError(req, res, err)
		return
	}
}

func (t *traitService) createComponentStorageItem(req *restful.Request, res *restful.Response) {
	app := req.Request.Context().Value(&apis.CtxKeyApplication).(*model.Application)
	var createReq apisCmb.StorageItemRequest
	if err := req.ReadEntity(&createReq); err != nil {
		bcode.ReturnError(req, res, err)
		return
	}

	if err := validate.Struct(&createReq); err != nil {
		bcode.ReturnError(req, res, err)
		return
	}

	trait, err := t.traitUsecase.CreateComponentStorageItem(req.Request.Context(), app,
		&model.ApplicationComponent{Name: req.PathParameter("compName")}, createReq)

	if err != nil {
		bcode.ReturnError(req, res, err)
		return
	}
	if err := res.WriteEntity(trait); err != nil {
		bcode.ReturnError(req, res, err)
		return
	}
}

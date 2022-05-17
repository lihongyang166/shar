package api

import (
	"context"
	"github.com/crystal-construct/shar/model"
	"github.com/crystal-construct/shar/server/health"
	"github.com/crystal-construct/shar/server/services"
	"github.com/crystal-construct/shar/server/workflow"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"sync"
)

type SharServer struct {
	model.SharServer
	log    *otelzap.Logger
	health *health.Checker
	store  services.Storage
	queue  services.Queue
	engine *workflow.Engine
}

func New(log *otelzap.Logger, store services.Storage, queue services.Queue) (*SharServer, error) {
	engine, err := workflow.NewEngine(log, store, queue)
	if err != nil {
		return nil, err
	}
	if err := engine.Start(context.Background()); err != nil {
		panic(err)
	}
	return &SharServer{
		log:    log,
		store:  store,
		queue:  queue,
		engine: engine,
	}, nil
}

func (s *SharServer) StoreWorkflow(ctx context.Context, process *model.Process) (*wrappers.StringValue, error) {
	res, err := s.engine.LoadWorkflow(ctx, process)
	return &wrappers.StringValue{Value: res}, err
}
func (s *SharServer) LaunchWorkflow(ctx context.Context, req *model.LaunchWorkflowRequest) (*wrappers.StringValue, error) {
	res, err := s.engine.Launch(ctx, req.Id, req.Vars)
	return &wrappers.StringValue{Value: res}, err
}

func (s *SharServer) CancelWorkflowInstance(ctx context.Context, req *model.CancelWorkflowInstanceRequest) (*empty.Empty, error) {
	err := s.engine.CancelWorkflowInstance(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &empty.Empty{}, err
}

var shutdownOnce sync.Once

func (s *SharServer) Shutdown() {
	shutdownOnce.Do(func() {
		s.engine.Shutdown()
	})
}

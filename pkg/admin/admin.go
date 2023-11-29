package admin

import (
	"context"
	"github.com/0fau/logs/pkg/database"
	"github.com/0fau/logs/pkg/database/sql"
	"github.com/0fau/logs/pkg/process"
	"github.com/0fau/logs/pkg/process/meter"
	"github.com/0fau/logs/pkg/s3"
	"github.com/cockroachdb/errors"
	"github.com/goccy/go-json"
	"google.golang.org/grpc"
	"log"
	"net"
	"slices"
	"sync"
)

var _ AdminServer = (*Server)(nil)

type Config struct {
	Address string
}

type Server struct {
	config *Config

	db        *database.DB
	s3        *s3.Client
	processor *process.Processor

	UnimplementedAdminServer
}

func NewServer(c *Config, db *database.DB, s3 *s3.Client, processor *process.Processor) *Server {
	return &Server{config: c, db: db, s3: s3}
}

func (s *Server) Run() error {
	lis, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return errors.Wrap(err, "listening on endpoint")
	}

	grpcServer := grpc.NewServer()
	RegisterAdminServer(grpcServer, s)

	if err := grpcServer.Serve(lis); err != nil {
		return errors.Wrap(err, "grpc serve")
	}
	return nil
}

func (s *Server) Process(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
	raw, err := s.s3.FetchEncounter(ctx, req.Encounter)
	if err != nil {
		return nil, errors.Wrap(err, "fetching encounter")
	}

	var enc *meter.Encounter
	if err := json.Unmarshal(raw, &enc); err != nil {
		return nil, errors.Wrap(err, "unmarshalling encounter")
	}

	proc, err := s.processor.Process(enc)
	if err != nil {
		return nil, errors.Wrap(err, "processing encounter")
	}

	if err := s.db.Queries.ProcessEncounter(ctx, sql.ProcessEncounterParams{
		ID:     req.Encounter,
		Header: proc.Header,
		Data:   proc.Data,
	}); err != nil {
		log.Println(errors.Wrap(err, "saving encounter"))
	}

	return &ProcessResponse{}, nil
}

func (s *Server) ProcessAll(ctx context.Context, req *ProcessAllRequest) (*ProcessAllResponse, error) {
	ids, err := s.db.Queries.ListEncounters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "listing encounter ids")
	}

	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	wg.Add(len(ids))

	for i := 0; i < len(ids); i++ {
		sem <- struct{}{}
		go func(enc int32) {
			defer wg.Done()
			s.Process(ctx, &ProcessRequest{Encounter: enc})
			<-sem
		}(ids[i])
	}

	wg.Wait()
	return &ProcessAllResponse{}, nil
}

func (s *Server) Role(ctx context.Context, req *RoleRequest) (*RoleResponse, error) {
	user, err := s.db.Queries.GetUser(ctx, req.Discord)
	if err != nil {
		return nil, errors.Wrap(err, "fetch user")
	}

	roles := user.Roles
	switch req.Action {
	case RoleRequest_Add:
		if slices.Contains(roles, req.Role) {
			return &RoleResponse{}, nil
		}
		roles = append(roles, req.Role)
	case RoleRequest_Remove:
		if !slices.Contains(roles, req.Role) {
			return &RoleResponse{}, nil
		}
		roles = slices.DeleteFunc(roles, func(role string) bool {
			return role == req.Role
		})
	}

	if err := s.db.Queries.SetUserRoles(ctx, sql.SetUserRolesParams{
		DiscordTag: req.Discord,
		Roles:      roles,
	}); err != nil {
		return nil, errors.Wrap(err, "setting roles")
	}

	return &RoleResponse{}, nil
}

func (s *Server) Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error) {
	if err := s.s3.DeleteEncounter(ctx, req.Encounter); err != nil {
		return nil, errors.Wrap(err, "s3 delete")
	}

	if err := s.db.Queries.DeleteEncounter(ctx, req.Encounter); err != nil {
		return nil, errors.Wrap(err, "db delete")
	}

	return &DeleteResponse{}, nil
}

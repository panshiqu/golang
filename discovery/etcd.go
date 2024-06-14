package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/panshiqu/golang/utils"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
)

type Service struct {
	cli  *clientv3.Client
	key  string
	addr string
}

func Register(uri string, key string, addr string, id int) (*Service, error) {
	slog.Info("register", slog.String("uri", uri), slog.String("key", key), slog.String("addr", addr), slog.Int("id", id))

	cli, err := clientv3.NewFromURL(uri)
	if err != nil {
		return nil, utils.Wrap(err)
	}

	resp, err := cli.Grant(context.Background(), 60)
	if err != nil {
		return nil, utils.Wrap(err)
	}

	data, err := json.Marshal(endpoints.Endpoint{
		Addr:     addr,
		Metadata: fmt.Sprint(id),
	})
	if err != nil {
		return nil, utils.Wrap(err)
	}

	if _, err = cli.Put(context.Background(), fmt.Sprintf("discovery/%s/%s", key, addr), string(data), clientv3.WithLease(resp.ID)); err != nil {
		return nil, utils.Wrap(err)
	}

	ch, err := cli.KeepAlive(context.Background(), resp.ID)
	if err != nil {
		return nil, utils.Wrap(err)
	}

	go func() {
		for v := range ch {
			slog.Debug("keepalive", slog.Any("lease", v.ID))
		}

		slog.Info("keepalive exit", slog.Any("lease", resp.ID))
	}()

	return &Service{
		cli:  cli,
		key:  key,
		addr: addr,
	}, nil
}

func (s *Service) Release() error {
	slog.Info("release", slog.String("key", s.key), slog.String("addr", s.addr))

	if _, err := s.cli.Delete(context.Background(), fmt.Sprintf("discovery/%s/%s", s.key, s.addr)); err != nil {
		return utils.Wrap(err)
	}

	return utils.Wrap(s.cli.Close())
}

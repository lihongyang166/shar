package upgradetest

import (
	"context"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func getContainers(ctx context.Context, natsVer string, sharVer string) (testcontainers.Container, testcontainers.Container, testcontainers.Network, error) {
	nw, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{testcontainers.NetworkRequest{
		CheckDuplicate: true,
		Name:           "services",
	}, testcontainers.ProviderDefault,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	natsContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{},
			Image:          natsImg(natsVer),
			ExposedPorts:   []string{"4222/tcp"},
			Cmd:            []string{"-js", "--server_name", "nats"},
			WaitingFor:     wait.ForLog("[INF] Server is ready"),
			Name:           "nats",
			Hostname:       "nats",
			Networks:       []string{"services"},
		},
		Started: true,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	sharContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        sharImg(sharVer),
			ExposedPorts: []string{"8080/tcp"},
			WaitingFor:   wait.ForLog("shar api listener started"),
			Networks:     []string{"services"},
			Env: map[string]string{
				"NATS_URL": "nats://nats:4222",
			},
		},
		Started: true,
	})
	return natsContainer, sharContainer, nw, nil
}

func killContainers(ctx context.Context, sharContainer testcontainers.Container, natsContainer testcontainers.Container, network testcontainers.Network) error {
	if err := sharContainer.Terminate(ctx); err != nil {
		return err
	}
	if err := natsContainer.Terminate(ctx); err != nil {
		return err
	}
	if err := network.Remove(ctx); err != nil {
		return err
	}
	return nil
}

func sharImg(version string) string {
	return "registry.gitlab.com/shar-workflow/shar/server:" + version
}

func natsImg(version string) string {
	return "nats:" + version
}

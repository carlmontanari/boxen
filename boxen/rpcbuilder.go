package boxen

import (
	"context"

	boxenprotov1 "github.com/carlmontanari/boxen/proto/v1"
	"gopkg.in/yaml.v3"
)

// Builder is rpc endpoint for the builder to ask stuff of the server.
func (b *Boxen) Builder(
	ctx context.Context,
	req *boxenprotov1.BuilderRequest,
) (*boxenprotov1.BuilderResponse, error) {
	_ = ctx

	b.l.Debug("builder request received")

	switch req.GetRequest().(type) { //nolint: gocritic
	case *boxenprotov1.BuilderRequest_PackageRequest:
		return b.sendPackageInfo()
	}

	// we just send empty response for log things of course
	return &boxenprotov1.BuilderResponse{}, nil
}

func (b *Boxen) sendPackageInfo() (*boxenprotov1.BuilderResponse, error) {
	d, err := yaml.Marshal(b.p)
	if err != nil {
		return nil, err
	}

	return &boxenprotov1.BuilderResponse{
		Response: &boxenprotov1.BuilderResponse_PackageResponse{
			PackageResponse: &boxenprotov1.PackageInfoResponse{
				Profile: d,
				Disk:    b.disk,
			},
		},
	}, nil
}

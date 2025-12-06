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

	switch req.GetRequest().(type) {
	case *boxenprotov1.BuilderRequest_PackageInfoRequest:
		return b.buildSendPackageInfo()
	case *boxenprotov1.BuilderRequest_PackageCompleteRequest:
		return b.buildProcessPackageDone()
	}

	// we just send empty response for log things of course
	return &boxenprotov1.BuilderResponse{}, nil
}

func (b *Boxen) buildSendPackageInfo() (*boxenprotov1.BuilderResponse, error) {
	d, err := yaml.Marshal(b.p)
	if err != nil {
		return nil, err
	}

	return &boxenprotov1.BuilderResponse{
		Response: &boxenprotov1.BuilderResponse_PackageInfoResponse{
			PackageInfoResponse: &boxenprotov1.PackageInfoResponse{
				Profile: d,
				Disk:    b.disk,
			},
		},
	}, nil
}

func (b *Boxen) buildProcessPackageDone() (*boxenprotov1.BuilderResponse, error) {
	b.agentDone <- struct{}{}

	return &boxenprotov1.BuilderResponse{
		Response: &boxenprotov1.BuilderResponse_PackageCompleteResponse{
			PackageCompleteResponse: &boxenprotov1.PackageCompleteResponse{},
		},
	}, nil
}

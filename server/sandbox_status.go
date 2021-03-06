package server

import (
	"encoding/json"

	"github.com/cri-o/cri-o/internal/oci"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// PodSandboxStatus returns the Status of the PodSandbox.
func (s *Server) PodSandboxStatus(ctx context.Context, req *pb.PodSandboxStatusRequest) (resp *pb.PodSandboxStatusResponse, err error) {
	sb, err := s.getPodSandboxFromRequest(req.PodSandboxId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "could not find pod %q: %v", req.PodSandboxId, err)
	}

	rStatus := pb.PodSandboxState_SANDBOX_NOTREADY
	if sb.Ready(true) {
		rStatus = pb.PodSandboxState_SANDBOX_READY
	}

	linux := &pb.LinuxPodSandboxStatus{
		Namespaces: &pb.Namespace{
			Options: &pb.NamespaceOption{
				Network: sb.NamespaceOptions().GetNetwork(),
				Ipc:     sb.NamespaceOptions().GetIpc(),
				Pid:     sb.NamespaceOptions().GetPid(),
			},
		},
	}

	sandboxID := sb.ID()
	resp = &pb.PodSandboxStatusResponse{
		Status: &pb.PodSandboxStatus{
			Id:          sandboxID,
			CreatedAt:   sb.CreatedAt().UnixNano(),
			Network:     &pb.PodSandboxNetworkStatus{},
			State:       rStatus,
			Labels:      sb.Labels(),
			Annotations: sb.Annotations(),
			Metadata:    sb.Metadata(),
			Linux:       linux,
		},
	}

	if len(sb.IPs()) > 0 {
		resp.Status.Network.Ip = sb.IPs()[0]
	}
	if len(sb.IPs()) > 1 {
		resp.Status.Network.AdditionalIps = toPodIPs(sb.IPs()[1:])
	}

	if req.Verbose {
		info, err := createSandboxInfo(sb.InfraContainer())
		if err != nil {
			return nil, errors.Wrap(err, "creating sandbox info")
		}
		resp.Info = info
	}

	return resp, nil
}

func toPodIPs(ips []string) (result []*pb.PodIP) {
	for _, ip := range ips {
		result = append(result, &pb.PodIP{Ip: ip})
	}
	return result
}

func createSandboxInfo(c *oci.Container) (map[string]string, error) {
	info := struct {
		Image       string    `json:"image"`
		Pid         int       `json:"pid"`
		RuntimeSpec spec.Spec `json:"runtimeSpec"`
	}{
		c.Image(),
		c.State().Pid,
		c.Spec(),
	}
	bytes, err := json.Marshal(info)
	if err != nil {
		return nil, errors.Wrapf(err, "marshal data: %v", info)
	}
	return map[string]string{"info": string(bytes)}, nil
}

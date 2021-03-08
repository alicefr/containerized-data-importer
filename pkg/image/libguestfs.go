package image

import (
	"github.com/alicefr/guestfs-server/libguestfs"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	log "k8s.io/klog/v2"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

func Sparsify(path string) error {
	addr := "unix://" + common.LibguestfsServerSocket
	opts = append(opts, grpc.WithInsecure())
	i := &libguestfs.Image{
		Path: path,
	}
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		log.Errorf("fail to dial: %v", err)
		return errors.Wrap(err, "Unable to connect to the libguestfs server")
	}
	defer conn.Close()
	client := libguestfs.NewVirtSparsifyClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = client.Sparsify(ctx, i)
	if err != nil {
		log.Errorf("Failed to sparsify image %s: %v", i.Path, err)
		return errors.Wrap(err, "Unable to sparsify the image")
	}
	return nil
}

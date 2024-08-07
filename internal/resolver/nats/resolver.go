package nats

import (
	"context"
	"strings"
	"sync"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/resolver"
)

// FIXME(alecmerdler): Make this configurable...
const (
	Scheme = "nats"

	addressesBucket = "spicedb"
	addressesKey    = "addresses"
)

type natsResolver struct {
	ctx    context.Context
	cancel context.CancelFunc
	// wg is used to enforce Close() to return after the watcher() goroutine has finished.
	wg sync.WaitGroup

	cc resolver.ClientConn
	nc *nats.Conn
}

var _ resolver.Resolver = &natsResolver{}

func (n *natsResolver) watch() error {
	js, err := n.nc.JetStream()
	if err != nil {
		return err
	}

	bucket, err := js.KeyValue(addressesBucket)
	if err != nil {
		return err
	}

	watcher, err := bucket.Watch(addressesKey, nats.IgnoreDeletes(), nats.IncludeHistory())
	if err != nil {
		return err
	}

	go func() {
		addressesUpdates := watcher.Updates()

		for {
			select {
			case <-n.ctx.Done():
				return
			case updates := <-addressesUpdates:
				// FIXME(alecmerdler): NATS will first emit all initial values from `Updates()`, need to ignore all but the most recent (will send a nil entry when all existing values are sent)...
				if updates == nil {
					continue
				}

				parsedAddresses := strings.Split(string(updates.Value()), ",")
				updatedAddresses := make([]resolver.Address, 0, len(parsedAddresses))
				for _, addr := range parsedAddresses {
					updatedAddresses = append(updatedAddresses, resolver.Address{
						Addr: addr,
					})
				}
				err := n.cc.UpdateState(resolver.State{
					Addresses: updatedAddresses,
				})
				if err != nil {
					return
				}
			}
		}
	}()

	return nil
}

func (n *natsResolver) ResolveNow(opts resolver.ResolveNowOptions) {
	// Since we are receiving updates to the bucket directly from NATS, we don't need to do anything here.
}

func (n *natsResolver) Close() {
	n.cancel()
	n.wg.Wait()
	n.nc.Close()
}

type builder struct {
	ctx context.Context
}

var _ resolver.Builder = &builder{}

func (b *builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	nc, err := nats.Connect(target.String())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(b.ctx)

	r := &natsResolver{
		ctx:    ctx,
		cancel: cancel,
		nc:     nc,
		cc:     cc,
	}
	if err := r.watch(); err != nil {
		return nil, err
	}

	return r, nil
}

func (b *builder) Scheme() string {
	return Scheme
}

// Register enables using the `nats://` scheme to resolve addresses stored in a NATS KV bucket.
func Register(ctx context.Context) error {
	resolver.Register(&builder{ctx})
	return nil
}

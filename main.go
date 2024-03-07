package main

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/client/rpc"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/traversal"
)

type BlockWrapper struct {
	api *rpc.HttpApi
}

// This wraps the api.Block().Get method with the correct type. It has almost
// the right signature, except for the second argument (string (actually a
// []byte) versus path.Path).
func (b BlockWrapper) Get(ctx context.Context, key string) ([]byte, error) {
	c, err := cid.Cast([]byte(key))
	if err != nil {
		return nil, err
	}

	p := path.FromCid(c)

	r, err := b.api.Block().Get(ctx, p)
	if err != nil {
		return nil, err
	}
	bytes, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// Same idea.
func (b BlockWrapper) Has(ctx context.Context, key string) (bool, error) {
	p, err := path.NewPath(key)
	if err != nil {
		return false, err
	}

	_, err = b.api.Block().Stat(ctx, p)
	if err != nil {
		return false, nil
	}

	return true, nil
}

func main() {

	// We open a connection to the local Kubo daemon. This object will provide access to the block store
	api, err := rpc.NewLocalApi()
	if err != nil {
		panic(err)
	}

	// to get bafyreidzmjkrqo...:
	//
	// echo  '["allo","ça","va"]' | ipfs dag put

	json := `
{
  "array": {
    "/": "bafyreidzmjkrqocbv7atv3ofkweydbjaihcbsziworhayptorjpwkoqfbm"
  },
  "truc": 12345
}`

	if err != nil {
		panic(err)
	}

	// We need something to pass to SetReadStorage so that the `Progress` (from
	// traversal) knows how to follow links.
	block := BlockWrapper{api}

	linksystem := cidlink.DefaultLinkSystem()

	linksystem.SetReadStorage(block)

	// A prototype which will use cidlink.Link{} and linking.LinkContext{} to
	// create Link nodes.
	prototype, err := basicnode.Chooser(cidlink.Link{}, linking.LinkContext{})
	if err != nil {
		panic(err)
	}

	// We make a node builder out of this prototype.
	builder := prototype.NewBuilder()

	// We decode the json into a ipld datamodel Node.
	err = dagjson.Decode(builder, strings.NewReader(json))

	// We get the built node
	node := builder.Build()

	// We need a progress object to pass to the traversal function the context
	// we want it to have. Here is the `LinkTargetNodePrototypeChooser` object
	// (responsible for choosing the node prototype to use for a subtree (to
	// help the system choose efficient datastructures for decoding child blocks
	// )), the linkSystem (useful for letting the traversal follow links) and
	// more.
	p := traversal.Progress{
		Cfg: &traversal.Config{
			LinkTargetNodePrototypeChooser: func(datamodel.Link, linking.LinkContext) (datamodel.NodePrototype, error) {
				return basicnode.Prototype.Any, nil
			},
			LinkSystem: linksystem,
		},
	}

	// In the ipld data model, we can assign to each subtree of a node a
	// specific path. Here, we can access this subobject(and follow its link)
	// without having to give it an explicit type.
	n, err := p.Get(node, datamodel.ParsePath("/array"))
	if err != nil {
		panic(err)
	}

	// Finally, we verify the result is as expected.
	dagjson.Encode(n, os.Stdout)
}

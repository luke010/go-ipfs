package commands

import (
	"fmt"
	"io"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"

	humanize "gx/ipfs/QmQMxG9D52TirZd9eLA37nxiNspnMRkKbyPWrVAa1gvtSy/go-humanize"
	cmds "gx/ipfs/QmQkW9fnCsg9SLHdViiAh6qfBppodsPZVpU92dZLqYtEfs/go-ipfs-cmds"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	bitswap "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap"
	decision "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/decision"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	cidutil "gx/ipfs/Qmf3gRH2L1QZy92gJHJEwKmBJKJGVf8RpN2kPPD2NQWg8G/go-cidutil"
)

var BitswapCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Interact with the bitswap agent.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"stat":      bitswapStatCmd,
		"wantlist":  showWantlistCmd,
		"ledger":    ledgerCmd,
		"reprovide": reprovideCmd,
	},
}

const (
	peerOptionName = "peer"
)

var showWantlistCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show blocks currently on the wantlist.",
		ShortDescription: `
Print out all blocks currently on the bitswap wantlist for the local peer.`,
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(peerOptionName, "p", "Specify which peer to show wantlist for. Default: self."),
	},
	Type: KeyList{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.OnlineMode() {
			return ErrNotOnline
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			return e.TypeErr(bs, nd.Exchange)
		}

		pstr, found := req.Options[peerOptionName].(string)
		if found {
			pid, err := peer.IDB58Decode(pstr)
			if err != nil {
				return err
			}
			if pid != nd.Identity {
				return cmds.EmitOnce(res, &KeyList{bs.WantlistForPeer(pid)})
			}
		}

		return cmds.EmitOnce(res, &KeyList{bs.GetWantlist()})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *KeyList) error {
			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}
			// sort the keys first
			cidutil.Sort(out.Keys)
			for _, key := range out.Keys {
				fmt.Fprintln(w, enc.Encode(key))
			}
			return nil
		}),
	},
}

const (
	bitswapVerboseOptionName = "verbose"
)

var bitswapStatCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Show some diagnostic information on the bitswap agent.",
		ShortDescription: ``,
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(bitswapVerboseOptionName, "v", "Print extra information"),
	},
	Type: bitswap.Stat{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.OnlineMode() {
			return cmdkit.Errorf(cmdkit.ErrClient, ErrNotOnline.Error())
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			return e.TypeErr(bs, nd.Exchange)
		}

		st, err := bs.Stat()
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, st)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, s *bitswap.Stat) error {
			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}
			verbose, _ := req.Options[bitswapVerboseOptionName].(bool)

			fmt.Fprintln(w, "bitswap status")
			fmt.Fprintf(w, "\tprovides buffer: %d / %d\n", s.ProvideBufLen, bitswap.HasBlockBufferSize)
			fmt.Fprintf(w, "\tblocks received: %d\n", s.BlocksReceived)
			fmt.Fprintf(w, "\tblocks sent: %d\n", s.BlocksSent)
			fmt.Fprintf(w, "\tdata received: %d\n", s.DataReceived)
			fmt.Fprintf(w, "\tdata sent: %d\n", s.DataSent)
			fmt.Fprintf(w, "\tdup blocks received: %d\n", s.DupBlksReceived)
			fmt.Fprintf(w, "\tdup data received: %s\n", humanize.Bytes(s.DupDataReceived))
			fmt.Fprintf(w, "\twantlist [%d keys]\n", len(s.Wantlist))
			for _, k := range s.Wantlist {
				fmt.Fprintf(w, "\t\t%s\n", enc.Encode(k))
			}

			fmt.Fprintf(w, "\tpartners [%d]\n", len(s.Peers))
			if verbose {
				for _, p := range s.Peers {
					fmt.Fprintf(w, "\t\t%s\n", p)
				}
			}

			return nil
		}),
	},
}

var ledgerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show the current ledger for a peer.",
		ShortDescription: `
The Bitswap decision engine tracks the number of bytes exchanged between IPFS
nodes, and stores this information as a collection of ledgers. This command
prints the ledger associated with a given peer.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peer", true, false, "The PeerID (B58) of the ledger to inspect."),
	},
	Type: decision.Receipt{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.OnlineMode() {
			return ErrNotOnline
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			return e.TypeErr(bs, nd.Exchange)
		}

		partner, err := peer.IDB58Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, bs.LedgerForPeer(partner))
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *decision.Receipt) error {
			fmt.Fprintf(w, "Ledger for %s\n"+
				"Debt ratio:\t%f\n"+
				"Exchanges:\t%d\n"+
				"Bytes sent:\t%d\n"+
				"Bytes received:\t%d\n\n",
				out.Peer, out.Value, out.Exchanged,
				out.Sent, out.Recv)
			return nil
		}),
	},
}

var reprovideCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Trigger reprovider.",
		ShortDescription: `
Trigger reprovider to announce our data to network.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.OnlineMode() {
			return ErrNotOnline
		}

		err = nd.Reprovider.Trigger(req.Context)
		if err != nil {
			return err
		}

		return nil
	},
}

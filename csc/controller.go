package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/codedellemc/gocsi"
	"github.com/codedellemc/gocsi/csi"
	"google.golang.org/grpc"
)

var controllerCmds = []*cmd{
	&cmd{
		Name:    "createvolume",
		Aliases: []string{"new", "create"},
		Action:  createVolume,
		Flags:   flagsCreateVolume,
	},
	&cmd{
		Name:    "deletevolume",
		Aliases: []string{"d", "rm", "del"},
		Action:  deleteVolume,
		Flags:   flagsDeleteVolume,
	},
	&cmd{
		Name:    "controllerpublishvolume",
		Aliases: []string{"att", "attach"},
		Action:  controllerPublishVolume,
		Flags:   flagsControllerPublishVolume,
	},
	&cmd{
		Name:    "controllerunpublishvolume",
		Aliases: []string{"det", "detach"},
		Action:  controllerUnpublishVolume,
		Flags:   flagsControllerUnpublishVolume,
	},
	&cmd{
		Name:    "validatevolumecapabilities",
		Aliases: []string{"v", "validate"},
		Action:  validateVolumeCapabilities,
		Flags:   flagsValidateVolumeCapabilities,
	},
	&cmd{
		Name:    "listvolumes",
		Aliases: []string{"l", "ls", "list"},
		Action:  listVolumes,
		Flags:   flagsListVolumes,
	},
	&cmd{
		Name:    "getcapacity",
		Aliases: []string{"getc", "capacity"},
		Action:  getCapacity,
		Flags:   flagsGetCapacity,
	},
	&cmd{
		Name:    "controllergetcapabilities",
		Aliases: []string{"cget"},
		Action:  controllerGetCapabilities,
		Flags:   flagsControllerGetCapabilities,
	},
}

///////////////////////////////////////////////////////////////////////////////
//                              CreateVolume                                 //
///////////////////////////////////////////////////////////////////////////////
var argsCreateVolume struct {
	reqBytes uint64
	limBytes uint64
	fsType   string
	mntFlags stringSliceArg
	params   mapOfStringArg
}

func flagsCreateVolume(ctx context.Context, rpc string) *flag.FlagSet {
	fs := flag.NewFlagSet(rpc, flag.ExitOnError)
	flagsGlobal(fs, volumeInfoFormat, "*csi.VolumeInfo")

	fs.Uint64Var(
		&argsCreateVolume.reqBytes,
		"requiredBytes",
		0,
		"The minimum volume size in bytes")

	fs.Uint64Var(
		&argsCreateVolume.limBytes,
		"limitBytes",
		0,
		"The maximum volume size in bytes")

	fs.StringVar(
		&argsCreateVolume.fsType,
		"t",
		"",
		"The file system type")

	fs.Var(
		&argsCreateVolume.mntFlags,
		"o",
		"The mount flags")

	fs.Var(
		&argsCreateVolume.params,
		"params",
		"Additional RPC parameters")

	fs.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"usage: %s %s [ARGS...] NAME\n",
			appName, rpc)
		fs.PrintDefaults()
	}

	return fs
}

func createVolume(
	ctx context.Context,
	fs *flag.FlagSet,
	cc *grpc.ClientConn) error {

	var (
		client csi.ControllerClient
		err    error
		tpl    *template.Template

		name     = fs.Arg(0)
		reqBytes = argsCreateVolume.reqBytes
		limBytes = argsCreateVolume.limBytes
		fsType   = argsCreateVolume.fsType
		mntFlags = argsCreateVolume.mntFlags.vals
		params   = argsCreateVolume.params.vals

		format  = args.format
		version = args.version
	)

	if name == "" {
		return &errUsage{"missing volume name"}
	}

	// create a template for emitting the output
	tpl = template.New("template")
	if tpl, err = tpl.Parse(format); err != nil {
		return err
	}

	// initialize the csi client
	client = csi.NewControllerClient(cc)

	// execute the rpc
	result, err := gocsi.CreateVolume(
		ctx, client, version, name,
		reqBytes, limBytes,
		fsType, mntFlags, params)
	if err != nil {
		return err
	}

	// emit the result
	if err = tpl.Execute(os.Stdout, result); err != nil {
		return err
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////
//                              DeleteVolume                                 //
///////////////////////////////////////////////////////////////////////////////
var argsDeleteVolume struct {
	volumeMD mapOfStringArg
}

func flagsDeleteVolume(ctx context.Context, rpc string) *flag.FlagSet {
	fs := flag.NewFlagSet(rpc, flag.ExitOnError)
	flagsGlobal(fs, "", "")

	fs.Var(
		&argsDeleteVolume.volumeMD,
		"metadata",
		"The metadata of the volume to be deleted.")

	fs.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"usage: %s %s [ARGS...] ID_KEY[=ID_VAL] [ID_KEY[=ID_VAL]...]\n",
			appName, rpc)
		fs.PrintDefaults()
	}

	return fs
}

func deleteVolume(
	ctx context.Context,
	fs *flag.FlagSet,
	cc *grpc.ClientConn) error {

	if fs.NArg() == 0 {
		return &errUsage{"missing volume ID"}
	}

	var (
		client csi.ControllerClient
		err    error

		volumeMD *csi.VolumeMetadata
		volumeID = &csi.VolumeID{Values: map[string]string{}}

		version = args.version
	)

	// parse the volume ID into a map
	for x := 0; x < fs.NArg(); x++ {
		a := fs.Arg(x)
		kv := strings.SplitN(a, "=", 2)
		switch len(kv) {
		case 1:
			volumeID.Values[kv[0]] = ""
		case 2:
			volumeID.Values[kv[0]] = kv[1]
		}
	}

	// initialize the csi client
	client = csi.NewControllerClient(cc)

	// execute the rpc
	err = gocsi.DeleteVolume(ctx, client, version, volumeID, volumeMD)
	if err != nil {
		return err
	}

	fmt.Println("Success")

	return nil
}

///////////////////////////////////////////////////////////////////////////////
//                          ControllerPublishVolume                          //
///////////////////////////////////////////////////////////////////////////////
var argsControllerPublishVolume struct {
	volumeMD mapOfStringArg
	nodeID   mapOfStringArg
	readOnly bool
}

func flagsControllerPublishVolume(
	ctx context.Context, rpc string) *flag.FlagSet {

	fs := flag.NewFlagSet(rpc, flag.ExitOnError)
	flagsGlobal(fs, mapSzOfSzFormat, "map[string]string")

	fs.Var(
		&argsControllerPublishVolume.volumeMD,
		"metadata",
		"The metadata of the volume to be used on a node.")

	fs.Var(
		&argsControllerPublishVolume.nodeID,
		"nodeID",
		"The ID of the node to which the volume should be published.")

	fs.BoolVar(
		&argsControllerPublishVolume.readOnly,
		"ro",
		false,
		"A flag indicating whether or not to "+
			"publish the volume in read-only mode.")

	fs.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"usage: %s %s [ARGS...] ID_KEY[=ID_VAL] [ID_KEY[=ID_VAL]...]\n",
			appName, rpc)
		fs.PrintDefaults()
	}

	return fs
}

func controllerPublishVolume(
	ctx context.Context,
	fs *flag.FlagSet,
	cc *grpc.ClientConn) error {

	if fs.NArg() == 0 {
		return &errUsage{"missing volume ID"}
	}

	var (
		client csi.ControllerClient
		err    error
		tpl    *template.Template

		volumeMD *csi.VolumeMetadata
		nodeID   *csi.NodeID

		volumeID = &csi.VolumeID{Values: map[string]string{}}
		readOnly = argsControllerPublishVolume.readOnly

		format  = args.format
		version = args.version
	)

	// parse the volume ID into a map
	for x := 0; x < fs.NArg(); x++ {
		a := fs.Arg(x)
		kv := strings.SplitN(a, "=", 2)
		switch len(kv) {
		case 1:
			volumeID.Values[kv[0]] = ""
		case 2:
			volumeID.Values[kv[0]] = kv[1]
		}
	}

	// check for volume metadata
	if v := argsControllerPublishVolume.volumeMD.vals; len(v) > 0 {
		volumeMD = &csi.VolumeMetadata{Values: v}
	}

	// check for a node ID
	if v := argsControllerPublishVolume.nodeID.vals; len(v) > 0 {
		nodeID = &csi.NodeID{Values: v}
	}

	// create a template for emitting the output
	tpl = template.New("template")
	if tpl, err = tpl.Parse(format); err != nil {
		return err
	}

	// initialize the csi client
	client = csi.NewControllerClient(cc)

	// execute the rpc
	result, err := gocsi.ControllerPublishVolume(
		ctx, client, version, volumeID,
		volumeMD, nodeID, readOnly)
	if err != nil {
		return err
	}

	// emit the result
	if err = tpl.Execute(os.Stdout, result.GetValues()); err != nil {
		return err
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////
//                        ControllerUnpublishVolume                          //
///////////////////////////////////////////////////////////////////////////////
var argsControllerUnpublishVolume struct {
	volumeMD mapOfStringArg
	nodeID   mapOfStringArg
}

func flagsControllerUnpublishVolume(
	ctx context.Context, rpc string) *flag.FlagSet {

	fs := flag.NewFlagSet(rpc, flag.ExitOnError)
	flagsGlobal(fs, "", "")

	fs.Var(
		&argsControllerUnpublishVolume.volumeMD,
		"metadata",
		"The metadata of the volume.")

	fs.Var(
		&argsControllerUnpublishVolume.nodeID,
		"nodeID",
		"The ID of the node on which the volume is published.")

	fs.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"usage: %s %s [ARGS...] ID_KEY[=ID_VAL] [ID_KEY[=ID_VAL]...]\n",
			appName, rpc)
		fs.PrintDefaults()
	}

	return fs
}

func controllerUnpublishVolume(
	ctx context.Context,
	fs *flag.FlagSet,
	cc *grpc.ClientConn) error {

	if fs.NArg() == 0 {
		return &errUsage{"missing volume ID"}
	}

	var (
		client csi.ControllerClient

		volumeMD *csi.VolumeMetadata
		nodeID   *csi.NodeID

		volumeID = &csi.VolumeID{Values: map[string]string{}}

		version = args.version
	)

	// parse the volume ID into a map
	for x := 0; x < fs.NArg(); x++ {
		a := fs.Arg(x)
		kv := strings.SplitN(a, "=", 2)
		switch len(kv) {
		case 1:
			volumeID.Values[kv[0]] = ""
		case 2:
			volumeID.Values[kv[0]] = kv[1]
		}
	}

	// check for volume metadata
	if v := argsControllerUnpublishVolume.volumeMD.vals; len(v) > 0 {
		volumeMD = &csi.VolumeMetadata{Values: v}
	}

	// check for a node ID
	if v := argsControllerUnpublishVolume.nodeID.vals; len(v) > 0 {
		nodeID = &csi.NodeID{Values: v}
	}

	// initialize the csi client
	client = csi.NewControllerClient(cc)

	// execute the rpc
	err := gocsi.ControllerUnpublishVolume(
		ctx, client, version, volumeID, volumeMD, nodeID)
	if err != nil {
		return err
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////
//                        ValidateVolumeCapabilities                         //
///////////////////////////////////////////////////////////////////////////////
var argsValidateVolumeCapabilities struct {
	fsType   string
	mntFlags stringSliceArg
}

func flagsValidateVolumeCapabilities(
	ctx context.Context, rpc string) *flag.FlagSet {

	fs := flag.NewFlagSet(rpc, flag.ExitOnError)
	flagsGlobal(fs, valCapFormat,
		"*csi.ValidateVolumeCapabilitiesResponse_Result")

	fs.StringVar(
		&argsValidateVolumeCapabilities.fsType,
		"t",
		"",
		"The file system type")

	fs.Var(
		&argsValidateVolumeCapabilities.mntFlags,
		"o",
		"The mount flags")

	fs.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"usage: %s %s [ARGS...] ID_KEY[=ID_VAL] [ID_KEY[=ID_VAL]...]\n",
			appName, rpc)
		fs.PrintDefaults()
	}

	return fs
}

func validateVolumeCapabilities(
	ctx context.Context,
	fs *flag.FlagSet,
	cc *grpc.ClientConn) error {

	if fs.NArg() == 0 {
		return &errUsage{"missing volume ID"}
	}

	var (
		client csi.ControllerClient

		volumeID = &csi.VolumeID{Values: map[string]string{}}
		caps     []*csi.VolumeCapability
		fsType   = argsValidateVolumeCapabilities.fsType
		mntFlags = argsCreateVolume.mntFlags.vals

		format  = args.format
		tpl     *template.Template
		version = args.version
	)

	// parse the volume ID into a map
	for x := 0; x < fs.NArg(); x++ {
		a := fs.Arg(x)
		kv := strings.SplitN(a, "=", 2)
		switch len(kv) {
		case 1:
			volumeID.Values[kv[0]] = ""
		case 2:
			volumeID.Values[kv[0]] = kv[1]
		}
	}

	// put the volumeID into a volumeInfo struct
	info := &csi.VolumeInfo{
		Id: volumeID,
	}

	if fsType != "" {
		caps = append(caps,
			&csi.VolumeCapability{
				Value: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						FsType: fsType,
					},
				},
			})
	}

	if len(mntFlags) > 0 {
		caps = append(caps,
			&csi.VolumeCapability{
				Value: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						MountFlags: mntFlags,
					},
				},
			})
	}

	// initialize the csi client
	client = csi.NewControllerClient(cc)

	// execute the rpc
	res, err := gocsi.ValidateVolumeCapabilities(
		ctx, client, version, info, caps)
	if err != nil {
		return err
	}

	// create a template for emitting the output
	tpl = template.New("template")
	if tpl, err = tpl.Parse(format); err != nil {
		return err
	}

	// emit the results
	if err = tpl.Execute(
		os.Stdout, res); err != nil {
		return err
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////
//                              ListVolumes                                  //
///////////////////////////////////////////////////////////////////////////////
var argsListVolumes struct {
	startingToken string
	maxEntries    uint64
	paging        bool
}

func flagsListVolumes(ctx context.Context, rpc string) *flag.FlagSet {
	fs := flag.NewFlagSet(rpc, flag.ExitOnError)
	flagsGlobal(fs, volumeInfoFormat, "*csi.VolumeInfo")

	fs.StringVar(
		&argsListVolumes.startingToken,
		"startingToken",
		os.Getenv("CSI_STARTING_TOKEN"),
		"A token to specify where to start paginating")

	var evMaxEntries uint64
	if v := os.Getenv("CSI_MAX_ENTRIES"); v != "" {
		i, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			fmt.Fprintf(
				os.Stderr,
				"error: max entries not uint32: %v\n",
				err)
		}
		evMaxEntries = i
	}
	fs.Uint64Var(
		&argsListVolumes.maxEntries,
		"maxEntries",
		evMaxEntries,
		"The maximum number of entries to return")

	fs.BoolVar(
		&argsListVolumes.paging,
		"paging",
		false,
		"Enables automatic paging")

	fs.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"usage: %s %s [ARGS...]\n",
			appName, rpc)
		fs.PrintDefaults()
	}

	return fs
}

func listVolumes(
	ctx context.Context,
	fs *flag.FlagSet,
	cc *grpc.ClientConn) error {

	var (
		client     csi.ControllerClient
		err        error
		maxEntries uint32
		tpl        *template.Template
		wg         sync.WaitGroup

		chdone        = make(chan int)
		cherrs        = make(chan error)
		format        = args.format
		startingToken = argsListVolumes.startingToken
		version       = args.version
	)

	// make sure maxEntries doesn't exceed uin32
	if max := argsListVolumes.maxEntries; max > maxUint32 {
		return fmt.Errorf("error: max entries > uin32: %v", max)
	}
	maxEntries = uint32(argsListVolumes.maxEntries)

	// create a template for emitting the output
	tpl = template.New("template")
	if tpl, err = tpl.Parse(format); err != nil {
		return err
	}

	// initialize the csi client
	client = csi.NewControllerClient(cc)

	// the two channels chdone and cherrs are used to
	// track the status of the goroutines as well as
	// the presence of any errors that need to be
	// returned from this function
	wg.Add(1)
	go func() {
		wg.Wait()
		close(chdone)
	}()

	go func() {
		tok := startingToken
		for {
			vols, next, err := gocsi.ListVolumes(
				ctx,
				client,
				version,
				maxEntries,
				tok)
			if err != nil {
				cherrs <- err
				return
			}
			wg.Add(1)
			go func(vols []*csi.VolumeInfo) {
				for _, v := range vols {
					if err := tpl.Execute(os.Stdout, v); err != nil {
						cherrs <- err
						return
					}
				}
				wg.Done()
			}(vols)
			if !argsListVolumes.paging || next == "" {
				break
			}
			tok = next
		}
		wg.Done()
	}()

	select {
	case <-chdone:
	case err := <-cherrs:
		if err != nil {
			return err
		}
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////
//                              GetCapacity                                  //
///////////////////////////////////////////////////////////////////////////////
func flagsGetCapacity(
	ctx context.Context, rpc string) *flag.FlagSet {

	fs := flag.NewFlagSet(rpc, flag.ExitOnError)
	flagsGlobal(fs, "", "")

	fs.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"usage: %s %s [ARGS...]\n",
			appName, rpc)
		fs.PrintDefaults()
	}
	return fs
}

func getCapacity(
	ctx context.Context,
	fs *flag.FlagSet,
	cc *grpc.ClientConn) error {

	// initialize the csi client
	client := csi.NewControllerClient(cc)

	// execute the rpc
	cap, err := gocsi.GetCapacity(ctx, client, args.version)
	if err != nil {
		return err
	}

	// emit the results
	fmt.Printf("TotalCapcity: %v\n", cap)

	return nil
}

///////////////////////////////////////////////////////////////////////////////
//                              ControllerGetCapabilities                    //
///////////////////////////////////////////////////////////////////////////////
func flagsControllerGetCapabilities(
	ctx context.Context, rpc string) *flag.FlagSet {

	fs := flag.NewFlagSet(rpc, flag.ExitOnError)
	flagsGlobal(fs, capFormat, "*csi.ControllerGetCapabilitiesResponse_Result")

	fs.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"usage: %s %s [ARGS...]\n",
			appName, rpc)
		fs.PrintDefaults()
	}
	return fs
}

func controllerGetCapabilities(
	ctx context.Context,
	fs *flag.FlagSet,
	cc *grpc.ClientConn) error {

	// initialize the csi client
	client := csi.NewControllerClient(cc)

	// execute the rpc
	caps, err := gocsi.ControllerGetCapabilities(ctx, client, args.version)
	if err != nil {
		return err
	}

	// create a template for emitting the output
	tpl := template.New("template")
	if tpl, err = tpl.Parse(args.format); err != nil {
		return err
	}
	// emit the results
	for _, c := range caps {
		if err = tpl.Execute(
			os.Stdout, c); err != nil {
			return err
		}
	}

	return nil
}

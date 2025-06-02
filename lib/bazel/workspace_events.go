package bazel

import (
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strings"

	bpb "github.com/enfabrica/enkit/lib/bazel/proto"
	"github.com/enfabrica/enkit/lib/proto/delimited"

	"google.golang.org/protobuf/proto"

	"google.golang.org/protobuf/encoding/protojson"
)

type WorkspaceEvents struct {
	WorkspaceHashes map[string]uint32
}

// extractChecksums returns a sorted list of download hashes from a set of
// relevant workspace events.
func (w *Workspace) extractChecksums(events []*bpb.WorkspaceEvent) []string {
	var checksums []string
	for _, event := range events {
		switch e := event.GetEvent().(type) {
		case *bpb.WorkspaceEvent_DownloadEvent:
			if e.DownloadEvent.GetSha256() != "" {
				checksums = append(checksums, e.DownloadEvent.GetSha256())
			}
			if e.DownloadEvent.GetIntegrity() != "" {
				checksums = append(checksums, e.DownloadEvent.GetIntegrity())
			}
		case *bpb.WorkspaceEvent_DownloadAndExtractEvent:
			if e.DownloadAndExtractEvent.GetSha256() != "" {
				checksums = append(checksums, e.DownloadAndExtractEvent.GetSha256())
			}
			if e.DownloadAndExtractEvent.GetIntegrity() != "" {
				checksums = append(checksums, e.DownloadAndExtractEvent.GetIntegrity())
			}
		case *bpb.WorkspaceEvent_ExecuteEvent:
			arguments := e.ExecuteEvent.GetArguments()
			// TODO ENGPROD-1075: Migrate astore downloads to repository_ctx
			if len(arguments) == 2 && arguments[0] == "echo" {
				checksums = append(checksums, arguments[1])
				// There are lots of astore related checksums invocation started with `echo`:
				// context: "repository @@c-capnproto"
				// execute_event {
				// 	arguments: "echo"
				// 	arguments: "a758d771f9246a1880de37c8a29f69c25e925cb03ba2974f0ecf8806d7ba2737"
				// 	...
				// }
				// So we extract checksums here
			} else if len(arguments) > 5 && arguments[0] == "enkit" {
				// Astore downloads are present in workspace events log as:
				// context: "repository @@generic-latest-kernel"
				// execute_event {
				//   arguments: "enkit"
				//   arguments: "astore"
				//   arguments: "download"
				//   arguments: "--force-uid"
				//   arguments: "chq3vth43g35tgzy5aad22wcdf5quiqs"
				// ...
				// }
				// So extract uid as checksum here
				if arguments[1] == "astore" && arguments[2] == "download" && arguments[3] == "--force-uid" {
					checksums = append(checksums, arguments[4])
				}
			}
		case *bpb.WorkspaceEvent_OsEvent:
		case *bpb.WorkspaceEvent_DeleteEvent:
		case *bpb.WorkspaceEvent_RenameEvent:
		case *bpb.WorkspaceEvent_FileEvent:
		case *bpb.WorkspaceEvent_PatchEvent:
		case *bpb.WorkspaceEvent_ReadEvent:
		case *bpb.WorkspaceEvent_WhichEvent:
		case *bpb.WorkspaceEvent_TemplateEvent:
		case *bpb.WorkspaceEvent_SymlinkEvent:
			// We have nothing to do with these events.
			continue
		default:
			w.options.Log.Debugf("Unchecked workspace event type  type: %s", protojson.Format(event))
		}
	}
	sort.Strings(checksums)
	return checksums
}

func (w *Workspace) ConstructWorkspaceEvents(workspaceEvents map[string][]*bpb.WorkspaceEvent) *WorkspaceEvents {
	workspaceHashes := map[string]uint32{}

	for workspaceName, events := range workspaceEvents {
		checksums := w.extractChecksums(events)
		if len(checksums) == 0 {
			continue
		}
		h := fnv.New32()
		for _, checksum := range checksums {
			fmt.Fprint(h, checksum)
		}
		workspaceHashes[workspaceName] = h.Sum32()
	}

	return &WorkspaceEvents{
		WorkspaceHashes: workspaceHashes,
	}
}

func (w *Workspace) ParseWorkspaceEvents(workspaceLog *os.File) (*WorkspaceEvents, error) {
	workspaceEvents := map[string][]*bpb.WorkspaceEvent{}
	rdr := delimited.NewReader(workspaceLog)
	for buf, err := rdr.Next(); err == nil; buf, err = rdr.Next() {
		var event bpb.WorkspaceEvent
		if err := proto.Unmarshal(buf, &event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal WorkspaceEvent message: %w", err)
		}
		context := event.GetContext()
		var workspaceName string
		if strings.HasPrefix(context, "repository @@") {
			// Bazel 7 format
			workspaceName = context[len("repository @@"):]
		} else if strings.HasPrefix(context, "repository @") {
			// Bazel 6 format
			workspaceName = context[len("repository @"):]
		} else {
			w.options.Log.Debugf("Unknown workspace event context type: %s", protojson.Format(&event))
			continue
		}

		workspaceEvents[workspaceName] = append(workspaceEvents[context], &event)
	}
	return w.ConstructWorkspaceEvents(workspaceEvents), nil
}

package bazel

import (
	"fmt"

	"os"
	"strings"

	"hash/fnv"
	"log/slog"
	"sort"

	bpb "github.com/enfabrica/enkit/lib/bazel/proto"
	"github.com/enfabrica/enkit/lib/proto/delimited"

	"google.golang.org/protobuf/proto"

	"google.golang.org/protobuf/encoding/protojson"
)

type WorkspaceEvents struct {
	EventsMap		map[string][]*bpb.WorkspaceEvent
	WorkspaceHashes map[string]uint32
}

// extractChecksums returns a sorted list of download hashes from a set of
// relevant workspace events.
func extractChecksums(events []*bpb.WorkspaceEvent) []string {
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
			if len(e.ExecuteEvent.GetArguments()) == 2 && e.ExecuteEvent.GetArguments()[0] == "echo" {
				fmt.Printf("got checksum: %q\n", e.ExecuteEvent.GetArguments()[1])
				checksums = append(checksums, e.ExecuteEvent.GetArguments()[1])
			}
		default:
			slog.Debug("Unchecked workspace event type  type: %s", protojson.Format(event))
		}
	}
	sort.Strings(checksums)
	return checksums
}

func ParseWorkspaceEvents(workspaceLog *os.File) (*WorkspaceEvents, error) {
	workspaceEvents := map[string][]*bpb.WorkspaceEvent{}
	rdr := delimited.NewReader(workspaceLog)
	for buf, err := rdr.Next(); err == nil; buf, err = rdr.Next() {
		var event bpb.WorkspaceEvent
		if err := proto.Unmarshal(buf, &event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal WorkspaceEvent message: %w", err)
		}
		workspaceEvents[event.GetRule()] = append(workspaceEvents[event.GetRule()], &event)
	}
	workspaceHashes := map[string]uint32{}

	for context, events := range workspaceEvents {
		var workspaceName string
		if strings.HasPrefix(context, "repository @@") {
			workspaceName = context[len("repository @@"):]
		} else {
			return nil, fmt.Errorf("Unknown workspace events context type: %s", context)
		}

		checksums := extractChecksums(events)
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
		EventsMap: workspaceEvents,
		WorkspaceHashes: workspaceHashes,
	}, nil
}
package packet

import (
	"testing"

	"github.com/gopacket/gopacket/layers"
)

/*
{LSUpdate:

		{NumOfLSAs:1
		LSAs:[
			{
				LSAheader:{LSAge:718 LSType:1 LinkStateID:3232257793 AdvRouter:3232257793 LSSeqNumber:2147484338 LSChecksum:49296 Length:36 LSOptions:2}
				Content:{Flags:0 Links:1 Routers:[
					{Type:3 LinkID:3232257792 LinkData:4294967040 Metric:10}
				]}
			}
		]
	}
*/
func TestLSAChkSum(t *testing.T) {
	lsa := LSAdvertisement{
		LSAheader: LSAheader{
			LSAge:       718,
			LSType:      1,
			LinkStateID: 3232257793,
			AdvRouter:   3232257793,
			LSSeqNumber: 2147484338,
			LSChecksum:  49296,
			Length:      36,
			LSOptions:   2,
		},
		Content: V2RouterLSA{
			RouterLSAV2: layers.RouterLSAV2{
				Flags: 0,
				Links: 1,
			},
			Routers: []RouterV2{
				{
					RouterV2: layers.RouterV2{
						Type:     3,
						LinkID:   3232257792,
						LinkData: 4294967040,
						Metric:   10,
					},
				},
			},
		},
	}
	t.Logf("expecte h: %+v", lsa.LSAheader)
	t.Logf("expecte l: %+v", lsa.Content)
	buf := make([]byte, lsa.Size())
	err := lsa.SerializeToSizedBuffer(buf)
	if err != nil {
		t.Fatalf("failed to serialize lsa: %s", err)
	}
	if lsa.LSChecksum != 49296 {
		t.Logf("butGot h: %+v", lsa.LSAheader)
		t.Logf("butGot l: %+v", lsa.Content)
		t.Errorf("invalid checksum, expect 49296 but got %d", lsa.LSChecksum)
	}

}

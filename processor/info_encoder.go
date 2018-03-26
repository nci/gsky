package processor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"syscall"
)

type POSIXDescriptor struct {
	GID   uint32 `json:"gid"`
	Group string `json:"group"`
	UID   uint32 `json:"uid"`
	User  string `json:"user"`
	Size  int64  `json:"size"`
	Mode  string `json:"mode"`
	Type  string `json:"type"`
	INode uint64 `json:"inode"`
	MTime int64  `json:"mtime"`
	ATime int64  `json:"atime"`
	CTime int64  `json:"ctime"`
}

type PosixInfo struct {
	In    chan string
	Out   chan string
	Error chan error
}

type JSONEncoder struct {
	In    chan *GeoFile
	Out   chan []byte
	Error chan error
}

func NewJSONEncoder(errChan chan error) *JSONEncoder {
	return &JSONEncoder{
		In:    make(chan *GeoFile, 100),
		Out:   make(chan []byte, 100),
		Error: errChan,
	}
}

func (jp *JSONEncoder) Run() {
	defer close(jp.Out)

	for geoFile := range jp.In {
		out, err := json.Marshal(&geoFile)
		if err != nil {
			jp.Error <- err
			return
		}
		jp.Out <- []byte(fmt.Sprintf("%s\tgdal\t%s\n", geoFile.FileName, string(out)))

		finfo, err := os.Stat(geoFile.FileName)
		if err != nil {
			jp.Error <- err
			continue
		}
		var rec syscall.Stat_t
		err = syscall.Lstat(geoFile.FileName, &rec)
		if err != nil {
			jp.Error <- err
			continue
		}
		gid, err := user.LookupGroupId(fmt.Sprintf("%d", rec.Gid))
		if err != nil {
			jp.Error <- err
			continue
		}
		uid, err := user.LookupId(fmt.Sprintf("%d", rec.Uid))
		if err != nil {
			jp.Error <- err
			continue
		}

		descr := POSIXDescriptor{Group: gid.Name, User: uid.Username, INode: rec.Ino, MTime: rec.Mtim.Sec, CTime: rec.Ctim.Sec, ATime: rec.Atim.Sec, Size: finfo.Size(), Mode: finfo.Mode().String(), Type: "file", UID: rec.Uid, GID: rec.Gid}

		out, err = json.Marshal(&descr)
		if err != nil {
			jp.Error <- err
			return
		}

		jp.Out <- []byte(fmt.Sprintf("%s\tposix\t%s\n", geoFile.FileName, string(out)))
	}

}

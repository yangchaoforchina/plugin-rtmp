package rtmp

import (
	"context"
	"net/http"

	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/config"

	"time"

	"m7s.live/engine/v4/util"
)

type RTMPConfig struct {
	config.Publish
	config.Subscribe
	config.TCP
	config.Pull
	config.Push
	ChunkSize int
	KeepAlive bool //保持rtmp连接，默认随着stream的close而主动断开
}

func (c *RTMPConfig) OnEvent(event any) {
	switch v := event.(type) {
	case FirstConfig:
		if c.ListenAddr != "" {
			RTMPPlugin.Info("server rtmp start at", zap.String("listen addr", c.ListenAddr))
			go c.Listen(RTMPPlugin, c)
		}
		if c.PullOnStart {
			for streamPath, url := range c.PullList {
				if err := RTMPPlugin.Pull(streamPath, url, new(RTMPPuller), false); err != nil {
					RTMPPlugin.Error("pull", zap.String("streamPath", streamPath), zap.String("url", url), zap.Error(err))
				}
			}
		}
	case config.Config:
		RTMPPlugin.CancelFunc()
		if c.ListenAddr != "" {
			RTMPPlugin.Context, RTMPPlugin.CancelFunc = context.WithCancel(Engine)
			RTMPPlugin.Info("server rtmp start at", zap.String("listen addr", c.ListenAddr))
			go c.Listen(RTMPPlugin, c)
		}
	case SEpublish:
		for streamPath, url := range c.PushList {
			if streamPath == v.Stream.Path {
				if err := RTMPPlugin.Push(streamPath, url, new(RTMPPusher), false); err != nil {
					RTMPPlugin.Error("push", zap.String("streamPath", streamPath), zap.String("url", url), zap.Error(err))
				}
			}
		}
	case *Stream: //按需拉流
		if c.PullOnSubscribe {
			for streamPath, url := range c.PullList {
				if streamPath == v.Path {
					if err := RTMPPlugin.Pull(streamPath, url, new(RTMPPuller), false); err != nil {
						RTMPPlugin.Error("pull", zap.String("streamPath", streamPath), zap.String("url", url), zap.Error(err))
					}
					break
				}
			}
		}
	}
}

var conf = &RTMPConfig{
	ChunkSize: 4096,
	TCP:       config.TCP{ListenAddr: ":1935"},
}
var RTMPPlugin = InstallPlugin(conf)

func filterStreams() (ss []*Stream) {
	Streams.RLock()
	defer Streams.RUnlock()
	for _, s := range Streams.Map {
		switch s.Publisher.(type) {
		case *RTMPReceiver, *RTMPPuller:
			ss = append(ss, s)
		}
	}
	return
}

func (*RTMPConfig) API_list(w http.ResponseWriter, r *http.Request) {
	util.ReturnJson(filterStreams, time.Second, w, r)
}

func (*RTMPConfig) API_Pull(rw http.ResponseWriter, r *http.Request) {
	err := RTMPPlugin.Pull(r.URL.Query().Get("streamPath"), r.URL.Query().Get("target"), new(RTMPPuller), r.URL.Query().Has("save"))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
	} else {
		rw.Write([]byte("ok"))
	}
}

func (*RTMPConfig) API_Push(rw http.ResponseWriter, r *http.Request) {
	err := RTMPPlugin.Push(r.URL.Query().Get("streamPath"), r.URL.Query().Get("target"), new(RTMPPusher), r.URL.Query().Has("save"))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
	} else {
		rw.Write([]byte("ok"))
	}
}

package rtsp

import proto "github.com/golang/protobuf/proto"

// AddStream to DB
func AddStream(stream *Stream) error {
	bytes, err := proto.Marshal(stream)
	if nil != err {
		log.Errorf("Marshal stream [%v]", err)
		return err
	}
	cmd := db.HSet("stream", stream.ID, bytes)

	if nil != cmd.Err() {
		log.WithError(cmd.Err()).WithField("cmd", cmd.Args()).Error("redis")
		return cmd.Err()
	}

	return nil
}

// RemoveStream from DB
func RemoveStream(ID string) error {
	cmd := db.HDel("stream", ID)
	if err := cmd.Err(); nil != err {
		log.WithError(err).WithField("cmd", cmd.Args()).Error("redis")
		return ErrorDB
	}

	return nil
}

// GetAllStream stored in DB
func GetAllStream() (streams []*Stream, err error) {
	all := db.HGetAll("stream")

	if err = all.Err(); err != nil {
		return
	}

	for _, bytes := range all.Val() {
		stream := &Stream{}
		if err = proto.Unmarshal([]byte(bytes), stream); err != nil {
			return
		}
		streams = append(streams, stream)
	}

	return
}

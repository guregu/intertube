package tube

import (
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/guregu/dynamo"
	"golang.org/x/net/context"
	// "github.com/aws/aws-sdk-go/service/cloudfront/sign"
)

type File struct {
	ID     string `dynamo:",hash" index:"UserID-ID-index,range"`
	UserID int    `index:"UserID-ID-index,hash"`

	Size     int64
	Type     string
	Name     string
	Ext      string
	Time     time.Time
	LocalMod int64
	Queued   time.Time
	Started  time.Time
	Finished time.Time
	Ready    bool
	Deleted  bool

	TrackID string
}

func NewFile(userID int, filename string, size int64) File {
	now := time.Now().UTC()
	garb, err := randomString(8)
	if err != nil {
		panic(err)
	}
	f := File{
		ID:     strconv.FormatInt(now.UnixNano(), 36) + "-" + garb,
		UserID: userID,
		Name:   filename,
		Ext:    path.Ext(filename),
		Size:   size,
		Time:   now,
	}
	return f
}

func (f File) Create(ctx context.Context) error {
	files := dynamoTable("Files")
	// users := dynamoTable("Users")

	err := files.Put(f).If("attribute_not_exists('ID')").Run()
	return err

	// do this in Track instead
	// return users.Update("ID", f.UserID).
	// 	Add("Usage", f.Size).
	// 	If("attribute_exists('ID')").
	// 	Run()
}

func (f *File) Finish(ctx context.Context, contentType string, size int64) error {
	files := dynamoTable("Files")
	err := files.Update("ID", f.ID).
		Set("Ready", true).
		Set("Finished", time.Now().UTC()).
		Set("Size", size).
		Set("Type", contentType).
		If("attribute_exists('ID')").
		Value(f)
	return err
}

// dont use TODO delete
func (f File) Delete(ctx context.Context) error {
	files := dynamoTable("Files")
	users := dynamoTable("Users")
	// tx := db.WriteTx()
	// tx.Delete(files.Delete("ID", f.ID))
	err := files.Update("ID", f.ID).
		Set("Deleted", true).
		If("attribute_exists('ID')").Run()
	if err != nil {
		return err
	}
	return users.Update("ID", f.UserID).
		SetExpr("'Usage' = 'Usage' - ?", f.Size).
		If("attribute_exists('ID')").Run()
}

func (f *File) SetTrackID(tID string) error {
	files := dynamoTable("Files")
	return files.Update("ID", f.ID).
		Set("TrackID", tID).
		Value(f)
}

func (f *File) SetQueued(ctx context.Context, at time.Time) error {
	files := dynamoTable("Files")
	return files.Update("ID", f.ID).
		Set("Queued", at).
		ValueWithContext(ctx, f)
}

func (f *File) SetStarted(ctx context.Context, at time.Time) error {
	files := dynamoTable("Files")
	return files.Update("ID", f.ID).
		Set("Started", at).
		ValueWithContext(ctx, f)
}

func (f File) Path() string {
	return "up/" + f.ID
}

func (f File) Status() string {
	switch {
	case f.Ready, !f.Finished.IsZero():
		return "done"
	case !f.Started.IsZero():
		return "processing"
	case !f.Queued.IsZero():
		return "queued"
	default:
		return "uploading"
	}
}

func (f File) Glyph() string {
	switch f.Type {
	case "audio/mpeg", "audio/ogg", "audio/aac", "audio/opus", "audio/wave", "audio/wav",
		"audio/midi", "audio/x-midi":
		return "♫"
	case "video/mpeg", "video/ogg", "video/quicktime", "video/x-matroska",
		"video/x-msvideo", "video/mp2t", "video/3gpp", "video/3gpp2",
		"image/tiff":
		return "❀"
	case "text/plain", "application/msword", "application/rtf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "✎"
	case "application/zip", "application/gzip", "application/x-bzip", "application/x-bzip2",
		"application/vnd.rar", "application/x-tar", "application/x-7z-compressed":
		return "⬢"
	}
	return "❐"
}

func GetFile(ctx context.Context, id string) (File, error) {
	table := dynamoTable("Files")
	var f File
	err := table.Get("ID", id).Consistent(true).One(&f)
	return f, err
}

func GetFiles(ctx context.Context, ids ...string) ([]File, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	table := dynamoTable("Files")
	var files []File
	batch := table.Batch("ID").Get()
	for _, id := range ids {
		batch.And(dynamo.Keys{id})
	}
	iter := batch.Iter()
	var f File
	for iter.Next(&f) {
		if f.Deleted {
			fmt.Println("SKIPPING FILE", f)
			continue
		}
		files = append(files, f)
	}
	err := iter.Err()
	if err == dynamo.ErrNotFound {
		err = nil
	}
	return files, err
}

func GetFilesByUser(ctx context.Context, userID string) ([]File, error) {
	table := dynamoTable("Files")
	var files []File
	err := table.Get("UserID", userID).
		Index("UserID-ID-index").
		Filter("Deleted <> ?", true).
		Order(dynamo.Descending).
		All(&files)
	if err == dynamo.ErrNotFound {
		err = nil
	}
	return files, err
}

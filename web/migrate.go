package web

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/guregu/tag"
	"github.com/kurin/blazer/b2"

	"github.com/guregu/intertube/storage"
	"github.com/guregu/intertube/tube"
)

func MIGRATE_MAKEDUMPS() {
	ctx := context.Background()
	users, err := tube.GetAllUsers(ctx)
	if err != nil {
		panic(err)
	}
	for _, u := range users {
		if u.Tracks == 0 {
			continue
		}
		if err := tube.RecreateDump(ctx, u.ID, time.Now().UTC()); err != nil {
			panic(err)
		}
		fmt.Println("dumped", u.ID, u.Email)
	}
}

func MIGRATE_DELETE_STRIPEDATA() {
	ctx := context.Background()
	users, err := tube.GetAllUsers(ctx)
	if err != nil {
		panic(err)
	}
	for _, u := range users {
		if err := u.DeletePaymentInfo(ctx); err != nil {
			panic(err)
		}
		fmt.Println("reset", u.ID, u.Email)
	}
}

func MIGRATE_TOTALTRACKS2() {
	ctx := context.Background()

	fmt.Println("settinmg user totals")

	users, err := tube.GetAllUsers(ctx)
	if err != nil {
		panic(err)
	}

	for _, u := range users {
		ct, err := tube.CountTracks(ctx, u.ID)
		if err != nil {
			panic(err)
		}
		ct2, err := tube.CountTracks2(ctx, u.ID)
		// u.SetTracks(ctx, int(ct))
		fmt.Println("count:", u.ID, ct, ct2)
	}
}

func MIGRATE_TOTALTRACKS_old() {
	ctx := context.Background()

	fmt.Println("settinmg user totals")

	ct := make(map[int]int)

	iter := tube.GetALLTracks(ctx)
	var t tube.Track
	for iter.Next(&t) {
		ct[t.UserID] = ct[t.UserID] + 1
		fmt.Print(".")
	}
	if iter.Err() != nil {
		panic(iter.Err())
	}
	fmt.Println()
	fmt.Println(ct)

	for uid, n := range ct {
		u := tube.User{ID: uid}
		if err := u.SetTracks(ctx, n); err != nil {
			panic(err)
		}
	}
}

func MIGRATE_SORTID() {
	ctx := context.Background()

	fmt.Println("fixing sort IDs...")

	iter := tube.GetALLTracks(ctx)
	var t tube.Track
	for iter.Next(&t) {
		if t.SortID == t.SortKey() {
			fmt.Println("skipping", t.SortKey())
			continue
		}
		if err := t.RefreshSortID(ctx); err != nil {
			fmt.Println("ERROR", err, t.ID, t.UserID, t.SortKey())
			continue
		}
		fmt.Println("set", t.SortID, "/", t.UserID)
	}
	if iter.Err() != nil {
		panic(iter.Err())
	}
}

func MIGRATE_ALL() {
	ctx := context.Background()

	fmt.Println("~~~~ migrating !~~~~~")
	b2bucket, err := b2Client.Bucket(ctx, b2BucketName)
	if err != nil {
		panic(err)
	}

	iter := tube.GetALLTracks(ctx)

	fchan := make(chan tube.Track, 50)

	for I := 0; I < 20; I++ {
		go func() {
			for track := range fchan {
				fmt.Println("STARTING", track.UserID, track.ID)

				r, err := storage.AWS_FilesBucket.Get(track.S3Key())
				if err != nil {
					fmt.Println("s3 error", err)
					continue
				}
				disp := "attachment; filename*=UTF-8''" + url.PathEscape(track.Filename)
				obj := b2bucket.Object(track.B2Key())
				w := obj.NewWriter(ctx, b2.WithAttrsOption(&b2.Attrs{
					ContentType: mimetypeOfTrack(tag.FileType(track.Filetype)),
					Info: map[string]string{
						"b2-content-disposition": disp,
					},
					LastModified: track.LastModOrDate(),
				}))

				var buf bytes.Buffer

				sz, err := io.Copy(&buf, r)
				if err != nil {
					fmt.Println("BfCOPY ERROR", track.UserID, track.ID)
				} else {
					fmt.Println("buffed", track.B2Key(), sz)
				}
				r.Close()

				bufr := bytes.NewReader(buf.Bytes())
				sz, err = io.Copy(w, bufr)
				if err != nil {
					fmt.Println("COPY ERROR", track.UserID, track.ID)
				} else {
					fmt.Println("wrote", track.B2Key(), sz)
				}
				w.Close()
			}
		}()
	}

	var track tube.Track
	for iter.Next(&track) {
		fchan <- track
		// fmt.Println("track", track.UserID, track.ID)
		// r, err := storage.AWS_FilesBucket.Get(track.S3Key())
		// if err != nil {
		// 	fmt.Println("s3 error", err)
		// 	continue
		// }
		// disp := "attachment; filename*=UTF-8''" + url.PathEscape(track.Filename)
		// obj := b2bucket.Object(track.B2Key())
		// w := obj.NewWriter(ctx, b2.WithAttrsOption(&b2.Attrs{
		// 	ContentType: mimetypeOfTrack(tag.FileType(track.Filetype)),
		// 	Info: map[string]string{
		// 		"b2-content-disposition": disp,
		// 	},
		// 	LastModified: track.LastModOrDate(),
		// }))

		// sz, err := io.Copy(w, r)
		// if err != nil {
		// 	fmt.Println("COPY ERROR")
		// } else {
		// 	fmt.Println("wrote", track.B2Key(), sz)
		// }
		// r.Close()
		// w.Close()
	}
	close(fchan)
	if err := iter.Err(); err != nil {
		fmt.Println("ERROR!!!!", err)
	}
}

func MIGRATE_CHECK() {
	ctx := context.Background()

	fmt.Println("~~~~ migratingCHEK !~~~~~")
	b2bucket, err := b2Client.Bucket(ctx, b2BucketName)
	if err != nil {
		panic(err)
	}

	iter := tube.GetALLTracks(ctx)

	fchan := make(chan tube.Track, 100)

	for I := 0; I < 50; I++ {
		go func() {
			for track := range fchan {
				if track.UserID == 76 {
					// test account
					continue
				}
				// fmt.Println("STARTING", track.UserID, track.ID)

				awsHead, err := storage.AWS_FilesBucket.Head(track.S3Key())
				if err != nil {
					fmt.Println("AWS ERROR", track.UserID, track.ID, err)
					continue
				}

				b2Head, err := storage.FilesBucket.Head(track.B2Key())
				if err != nil {
					fmt.Println("\nB2 ERROR", track.UserID, track.ID, err)
					if !strings.Contains(err.Error(), "NotFound") {
						continue
					}
				}

				if b2Head.Size >= awsHead.Size {
					fmt.Print(".")
					// fmt.Println("ok:", track.UserID, track.ID, b2Head.Size, awsHead.Size)
					continue
				}

				r, err := storage.AWS_FilesBucket.Get(track.S3Key())
				if err != nil {
					fmt.Println("\ns3 error", err)
					continue
				}
				disp := "attachment; filename*=UTF-8''" + url.PathEscape(track.Filename)
				obj := b2bucket.Object(track.B2Key())
				w := obj.NewWriter(ctx, b2.WithAttrsOption(&b2.Attrs{
					ContentType: mimetypeOfTrack(tag.FileType(track.Filetype)),
					Info: map[string]string{
						"b2-content-disposition": disp,
					},
					LastModified: track.LastModOrDate(),
				}))

				var buf bytes.Buffer

				sz, err := io.Copy(&buf, r)
				if err != nil {
					fmt.Println("BfCOPY ERROR", track.UserID, track.ID)
					r.Close()
					continue
				} else {
					fmt.Println("buffed,fixn", track.B2Key(), sz)
				}
				r.Close()

				bufr := bytes.NewReader(buf.Bytes())
				sz, err = io.Copy(w, bufr)
				if err != nil {
					fmt.Println("COPY ERROR", track.UserID, track.ID)
					w.Close()
					continue
				} else {
					fmt.Println("fixed", track.B2Key(), sz)
				}
				w.Close()
			}
		}()
	}

	var track tube.Track
	for iter.Next(&track) {
		fchan <- track
	}
	close(fchan)
	if err := iter.Err(); err != nil {
		fmt.Println("ERROR!!!!", err)
	}
	fmt.Println("\ndone ^_^")
}

func MIGRATE_PICS2() {
	ctx := context.Background()

	fmt.Println("~~~~ pixchek !~~~~~")
	b2bucket, err := b2Client.Bucket(ctx, b2BucketName)
	if err != nil {
		panic(err)
	}

	iter := tube.GetALLTracks(ctx)

	fchan := make(chan tube.Track, 100)

	for I := 0; I < 50; I++ {
		go func() {
			for track := range fchan {
				if track.UserID == 76 {
					// test account
					continue
				}
				if track.Picture.ID == "" {
					continue
				}
				// fmt.Println("STARTING", track.UserID, track.ID)
				pic := track.Picture
				r, err := storage.AWS_FilesBucket.Get(pic.S3Key())
				if err != nil {
					fmt.Println("\ns3 error", err)
					continue
				}
				// disp := "attachment; filename*=UTF-8''" + url.PathEscape(track.Filename)
				obj := b2bucket.Object(pic.S3Key())
				w := obj.NewWriter(ctx, b2.WithAttrsOption(&b2.Attrs{
					// ContentType: mimetypeOfTrack(tag.FileType(track.Filetype)),
					// Info: map[string]string{
					// 	"b2-content-disposition": disp,
					// },
					LastModified: track.LastModOrDate(),
				}))

				var buf bytes.Buffer

				sz, err := io.Copy(&buf, r)
				if err != nil {
					fmt.Println("BfCOPY ERROR", track.UserID, track.ID)
					r.Close()
					continue
				} else {
					fmt.Println("buffed,fixn", pic.S3Key(), sz)
				}
				r.Close()

				bufr := bytes.NewReader(buf.Bytes())
				sz, err = io.Copy(w, bufr)
				if err != nil {
					fmt.Println("COPY ERROR", track.UserID, track.ID)
					w.Close()
					continue
				} else {
					fmt.Println("fixed", pic.S3Key(), sz)
				}
				w.Close()
			}
		}()
	}

	var track tube.Track
	for iter.Next(&track) {
		fchan <- track
	}
	close(fchan)
	if err := iter.Err(); err != nil {
		fmt.Println("ERROR!!!!", err)
	}
	fmt.Println("\ndone ^_^")
}

func MIGRATE_PICS() {
	pix, err := storage.AWS_FilesBucket.List("pic/")
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	for key, obj := range pix {
		wg.Add(1)
		key, obj := key, obj
		go func() {
			defer wg.Done()
			r, err := storage.AWS_FilesBucket.Get(key)
			if err != nil {
				fmt.Println("PIX ERROR", err)
				return
			}

			var buf bytes.Buffer
			_, err = io.Copy(&buf, r)
			r.Close()
			if err != nil {
				fmt.Println("PbfIX ERROR", err)
				return
			}

			var rdr = bytes.NewReader(buf.Bytes())

			err = storage.FilesBucket.Put(obj.Type, key, rdr)
			if err != nil {
				fmt.Println("put err", err)
			}
			fmt.Println("done", key)
		}()
	}
	wg.Wait()
}

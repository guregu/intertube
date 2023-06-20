package web

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/guregu/kami"
	"golang.org/x/net/context"
)

func PanicHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if isSubsonicReq(r) {
		subsonicPanicHandler(ctx, w, r)
		return
	}

	ex := kami.Exception(ctx)
	log.Println("Panic!", ex)
	debug.PrintStack()

	w.WriteHeader(http.StatusInternalServerError)

	fmt.Fprintln(w, "Panic!", ex)
}

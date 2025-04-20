package main

import (
	"fmt"
	"net/http"
	"testing"
)

var uploadFormTmpl = []byte(`
<html>
	<body>
	<form action="/upload" method="post" enctype="multipart/form-data">
		Image: <input type="file" name="my_file">
		<input type="submit" value="Upload">
	</form>
	</body>
</html>
`)

func TestMain(t *testing.T) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(uploadFormTmpl)
	})

	fmt.Println("starting server at :8080")
	http.ListenAndServe(":8080", nil)
}

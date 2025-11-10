PKGS       = ./...
COVER_OUT  = cover.out
COVER_HTML = cover.html

.PHONY: test cover cover-html cover-view clean

# Run all tests (fast)
test:
	go test $(PKGS)

# Generate coverage profile
cover:
	go test $(PKGS) -coverprofile=$(COVER_OUT)

# Generate HTML coverage report
cover-html: cover
	go tool cover -html=$(COVER_OUT) -o $(COVER_HTML)

# Generate + open in browser (Linux: xdg-open)
cover-view: cover-html
	xdg-open $(COVER_HTML)

clean:
	rm -f $(COVER_OUT) $(COVER_HTML)

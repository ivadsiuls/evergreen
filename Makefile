PREFIX ?= $(HOME)/.local
SDL_CFLAGS := $(shell pkg-config --cflags sdl3)
SDL_LIBS := $(shell pkg-config --libs sdl3)

.PHONY: all install test
all: evergreen native/input-filter.so

evergreen: $(wildcard *.go util/*.go) go.mod go.sum
	go build -o $@ .

native/input-filter.so: native/input-filter.c
	$(CC) -std=c11 -Wall -Wextra -Werror -O2 -fPIC -shared $(SDL_CFLAGS) -o $@ $< $(SDL_LIBS)

install: all
	install -Dm755 evergreen $(PREFIX)/bin/evergreen
	install -Dm755 native/input-filter.so $(PREFIX)/lib/evergreen/input-filter.so
	install -Dm644 assets/evergreen.desktop $(PREFIX)/share/applications/evergreen.desktop
	install -Dm644 assets/evergreen.svg $(PREFIX)/share/icons/hicolor/scalable/apps/evergreen.svg

test: native/input-filter.so native/test-bin/rosemary
	go test ./...
	LD_PRELOAD=$(CURDIR)/native/input-filter.so ./native/test-bin/rosemary
	cp native/test-bin/rosemary native/test-bin/helper
	LD_PRELOAD=$(CURDIR)/native/input-filter.so ./native/test-bin/helper unfiltered

native/test-bin/rosemary: native/input-filter-test.c
	mkdir -p native/test-bin
	$(CC) -std=c11 -D_GNU_SOURCE -Wall -Wextra -Werror -O2 $(SDL_CFLAGS) -o $@ $< $(SDL_LIBS)

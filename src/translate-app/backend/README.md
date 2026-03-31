# Backend (Wails + Go)

Frontend source lives in **`../frontend/`** (sibling folder). Vite writes production assets to **`dist/`** in this directory (for `go:embed`).

## About

Wails v2 + Go. See also [../README.md](../README.md).

You can configure the project by editing `wails.json`. More information about the project settings can be found
here: https://wails.io/docs/reference/project-config

## Live Development

From this directory, run `wails dev`. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.

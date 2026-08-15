# movieswipe

Tinder-style movie picker for two phones. One person creates a room and shares
a 6-character code; both people swipe through the same deck of movies on
their own phones, whenever they like, and the app surfaces the intersection —
the films you both said yes to.

[![Test](https://github.com/brk3/movieswipe/actions/workflows/test.yml/badge.svg)](https://github.com/brk3/movieswipe/actions/workflows/test.yml)
[![Docker Build](https://github.com/brk3/movieswipe/actions/workflows/docker-latest.yml/badge.svg)](https://github.com/brk3/movieswipe/actions/workflows/docker-latest.yml)

## Development

```bash
export TMDB_TOKEN=...   # TMDB Settings -> API -> API Read Access Token (v4 bearer token)
export OMDB_API_KEY=... # from omdbapi.com

make run
```

Then open `http://localhost:8080`.

```bash
make test    # go test -cover ./...
make build   # dist/movieswipe
```

## Configuration

Env vars, no config file:

| Var             | Default               | Required                    |
|------------------|-----------------------|------------------------------|
| `ADDR`           | `:8080`                | no                          |
| `DB_PATH`        | `./movieswipe.db`      | no                          |
| `TMDB_TOKEN`     | —                      | yes, for discovery/posters  |
| `OMDB_API_KEY`   | —                      | no, ratings only            |

Without `TMDB_TOKEN` the server still starts, but genres and cards stay empty.

## Deploying

The binary is a single container: it serves the API and the embedded
`web/` frontend, backed by a SQLite database at `DB_PATH`.

```bash
# throwaway smoke test on any cluster
cp deploy/kustomize/dev/.env.example deploy/kustomize/dev/.env
# fill in TMDB_TOKEN / OMDB_API_KEY in that .env file
kubectl apply -k deploy/kustomize/dev
kubectl -n movieswipe port-forward svc/movieswipe 8080:80
```

`deploy/kustomize/base` is the reusable base referenced by cluster-specific
overlays elsewhere (see the `homelab` repo for the `dagda` deployment,
fronted by envoy-gateway at `movieswipe.aiectomy.xyz`).

## Architecture

- Go stdlib `net/http.ServeMux` for routing, no framework.
- `modernc.org/sqlite` (pure Go) for storage — `CGO_ENABLED=0` static builds.
- `internal/tmdb` and `internal/omdb` are thin clients; `internal/catalog`
  is the only thing deciding which TMDB ids enter a room's deck, and the
  seam for a future AI-backed movie source.
- Auth is an `X-Member-Token` header issued on room create/join, checked
  against the room in the URL.

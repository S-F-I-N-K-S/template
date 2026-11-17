set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

move() {
	local src="$1" dst="_legacy/$1"
	if [ -e "$src" ]; then
		mkdir -p "$(dirname "$dst")"
		git mv "$src" "$dst"
		echo "moved: $src -> $dst"
	fi
}

move internal/service
move internal/transport
move internal/storage
move internal/txmanager
move internal/repository/trip_repository.go
move internal/repository/history_repository.go

echo
echo "Дальше: go build ./... (go.mod: go get/go mod tidy делай, когда в коде появятся импорты OTel, testify, gomock — иначе tidy их удалит)"

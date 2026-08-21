# Maintainer: Jashk120 <jashk120@example.com>
pkgname=rambo
pkgver=0.1.0
pkgrel=1
pkgdesc="Kernel-backed, policy-driven RAM monitor and OOM killer daemon"
arch=('x86_64' 'aarch64')
url="https://github.com/Jashk120/Rambo"
license=('MIT')
depends=('polkit')
makedepends=('go')
optdepends=('libnotify: desktop notifications'
            'systemd: user service')
provides=('rambo')
conflicts=('rambo-git')
source=("$pkgname-$pkgver.tar.gz::https://github.com/Jashk120/Rambo/archive/v$pkgver.tar.gz")
sha256sums=('SKIP')

build() {
  cd "Rambo-$pkgver"
  export CGO_ENABLED=0
  export GOFLAGS="-trimpath -buildmode=pie -mod=readonly -modcacherw"
  local _commit _date
  _commit=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
  _date=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  go build \
    -ldflags "-s -w -X github.com/jashk120/rambo/internal/version.Version=v$pkgver -X github.com/jashk120/rambo/internal/version.Commit=$_commit -X github.com/jashk120/rambo/internal/version.Date=$_date" \
    -o rambo .
}

check() {
  cd "Rambo-$pkgver"
  go test ./...
}

package() {
  cd "Rambo-$pkgver"
  install -Dm755 rambo "$pkgdir/usr/bin/rambo"
  install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
  install -Dm644 polkit/99-rambo.rules "$pkgdir/usr/share/polkit-1/rules.d/99-rambo.rules"
  install -Dm644 systemd/rambo.service "$pkgdir/usr/lib/systemd/user/rambo.service"
  install -Dm644 docs/rambo.1 "$pkgdir/usr/share/man/man1/rambo.1"
  install -Dm644 README.md "$pkgdir/usr/share/doc/$pkgname/README.md"

  # Shell completions (generated from the built binary)
  install -d "$pkgdir/usr/share/bash-completion/completions"
  install -d "$pkgdir/usr/share/zsh/site-functions"
  install -d "$pkgdir/usr/share/fish/vendor_completions.d"
  ./rambo completion bash > "$pkgdir/usr/share/bash-completion/completions/rambo"
  ./rambo completion zsh  > "$pkgdir/usr/share/zsh/site-functions/_rambo"
  ./rambo completion fish > "$pkgdir/usr/share/fish/vendor_completions.d/rambo.fish"
}

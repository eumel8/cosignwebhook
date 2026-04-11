{ pkgs ? import (fetchTarball "https://github.com/NixOS/nixpkgs/archive/nixpkgs-unstable.tar.gz") { } }:

let
  cosign-v3 = pkgs.runCommand "cosign-v3" { } ''
    mkdir -p $out/bin
    ln -s ${pkgs.cosign}/bin/cosign $out/bin/cosign-v3
  '';
in
pkgs.mkShell {
  packages = [
    cosign-v3
    pkgs.go
    pkgs.k3d
    pkgs.kubectl
    pkgs.kubernetes-helm
    pkgs.docker
  ];

  shellHook = ''
    # Use the macOS system DNS resolver so .localhost subdomains resolve
    # via mDNSResponder (the pure-Go resolver cannot handle these on macOS)
    export GODEBUG="netdns=cgo"

    # Use port 5100 on macOS to avoid conflict with AirPlay Receiver on port 5000
    export HOST_PORT="5100"

    echo "cosignwebhook dev shell"
    echo "  cosign-v3: $(cosign-v3 version 2>&1 | head -1)"
    echo "  go:        $(go version)"
    echo "  k3d:       $(k3d version | head -1)"
    echo "  kubectl:   $(kubectl version --client -o yaml 2>/dev/null | grep gitVersion | head -1)"
    echo "  helm:      $(helm version --short)"
  '';
}

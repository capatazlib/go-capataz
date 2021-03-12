{ pkgs, go  }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    go
    go-capataz

    # linting
    gotools godef golint golangci-lint
  ];

  shellHook = ''
    unset GOROOT
    unset GOPATH
  '';
}

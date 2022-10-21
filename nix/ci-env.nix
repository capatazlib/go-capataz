system: { self, ... }: { mkShell, go_1_19, gotools, godef, golint, golangci-lint }:

mkShell {
  buildInputs = (builtins.attrValues {
    inherit (self.packages.${system}) go-capataz;
  }) ++ [ go_1_19 gotools godef golint golangci-lint ];

  shellHook = ''
    unset GOROOT
    unset GOPATH
  '';
}

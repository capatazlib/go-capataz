system: {self, ...}: {
  mkShell,
  figlet,
  stdenv,
  cacert,
  go_1_19,
  gotools,
  godef,
  gocode,
  golint,
  golangci-lint,
  gogetdoc,
  gopkgs,
  gotests,
  impl,
  errcheck,
  reftools,
  delve,
  gopls,
  gomod2nix
}:

mkShell {
  buildInputs = [
    # bash scripts utilities
    figlet
    stdenv
    cacert

    go_1_19

    # recommended packages to have for development with emacs/spacemacs
    gotools godef gocode golint golangci-lint gogetdoc gopkgs gotests impl
    errcheck reftools self.packages.${system}.humanlog delve gopls gomod2nix
  ];

  shellHook = ''
  unset GOPATH
  unset GOROOT
  '';
}

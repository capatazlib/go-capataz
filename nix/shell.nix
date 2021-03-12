{ pkgs, go }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    # bash scripts utilities
    figlet
    stdenv
    cacert

    go

    # recommended packages to have for development with emacs/spacemacs
    gotools godef gocode golint golangci-lint gogetdoc gopkgs gotests impl
    errcheck reftools humanlog delve gopls gomod2nix

    # capataz deps
    go-capataz
  ];

  shellHook = ''
  unset GOPATH
  unset GOROOT
  '';
}

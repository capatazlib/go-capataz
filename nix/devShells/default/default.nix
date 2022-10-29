system: {self, ...}: {
  mkShell,
  figlet,
  lolcat,
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
  gomod2nix,
}: let
  pre-commit-hook =
    (self.lib.pre-commit system).shellHook;
in
  mkShell {
    buildInputs = [
      # bash scripts utilities
      figlet
      lolcat

      self.packages.${system}.dev-env
      self.packages.${system}.humanlog

      delve
      gopls

      gomod2nix
    ];

    shellHook =
      "figlet go-capataz | lolcat -t -p 2;"
      + pre-commit-hook;
  }

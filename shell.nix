let
  pkgs = import <nixpkgs> {};

  humanlog = with pkgs; buildGoPackage rec {
    name = "humanlog";
    version = "0.2.1";
    rev = "0.2.1";
    goPackagePath = "github.com/aybabtme/humanlog";
    src = fetchFromGitHub {
      inherit rev;
      owner = "aybabtme";
      repo = "humanlog";
      sha256 = "081r0fw0v0lk75lyvmlq3a8xnz8nyzs0a2090dd1lgj51ki8bm9r";
    };
    meta = with lib; {
      description = "Get human readable logs";
      homepage = "https://github.com/aybabtme/humanlog";
      license = licenses.asl20;
      platforms = platforms.linux;
    };
  };

in

  pkgs.mkShell {
    buildInputs = with pkgs; [
      # bash scripts utilities
      stdenv
      figlet

      # current go version
      # go_1_12

      # miscellaneous gathering of data
      jq     # json
      miller # csv

      # recommended packages to have for development with emacs/spacemacs
      gotools godef gocode golint golangci-lint gogetdoc gopkgs gotests impl
      errcheck reftools humanlog delve
    ];

    # CFLAGS="-I${pkgs.glibc.dev}/include -I${pkgs.libvirt}/include";
    # LDFLAGS="-L${pkgs.glibc}/lib -L${pkgs.libvirt}/lib";

    shellHook = ''
      figlet "go-Capataz"
      source .env.sh
      echo "GOPATH: ''${GOPATH}"
      go version
    '';
  }

{ stdenv, go, buildGoModule, fetchgit }:

buildGoModule rec {
  pname = "gopls";
  version = "go-capataz";
  rev = "72e4a01eba4315301fd9ce00c8c2f492580ded8a";

  src = fetchgit {
    inherit rev;
    url = "https://go.googlesource.com/tools";
    sha256 = "0a8c7j4w784w441j3j3bh640vy1g6g214641qv485wyi0xj49anf";
  };

  modRoot = "./gopls";

  vendorSha256 = "06w6qsya16d3zx552advlqbm1hgjqjdy765yafp8p2i2xvmgpyma";

  # Set GOTOOLDIR for derivations adding this to buildInputs
  # postInstall = ''
  #   mkdir -p $out/nix-support
  #   substitute ${../../go-modules/tools/setup-hook.sh} $out/nix-support/setup-hook \
  #     --subst-var-by bin $out
  # '';

  # Do not copy this without a good reason for enabling
  # In this case tools is heavily coupled with go itself and embeds paths.
  allowGoReference = true;
}

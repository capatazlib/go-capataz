system: inputs: {
  lib,
  buildGoPackage,
  fetchFromGitHub,
}: let
  rev = "0.6.0";
in
  buildGoPackage {
    name = "humanlog";
    version = "0.6.0";
    inherit rev;

    goPackagePath = "github.com/aybabtme/humanlog";
    src = fetchFromGitHub {
      inherit rev;
      owner = "humanlogio";
      repo = "humanlog";
      sha256 = "sha256-oaGqrGLvMq0To8fBV1sgRnC21xsaxG0fgVSX22Ibuvw=";
    };
    meta = with lib; {
      description = "Logs for humans to read.";
      homepage = "https://github.com/humanlogio/humanlog";
      license = licenses.asl20;
      platforms = platforms.x86_64 ++ platforms.aarch64;
    };
  }

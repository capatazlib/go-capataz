let
  sources = import ./nix/sources.nix {};
  pinnedPkgs = import sources.nixpkgs {};

in

{ pkgs ? pinnedPkgs }:

assert pkgs.lib.versionAtLeast pkgs.go.version "1.14";

pkgs.buildGoModule rec {
  name = "go-capataz";
  version = "latest";
  src = ./.;
  vendorSha256 = "0zv0nfbc1qkj5jq9ax3da7g7jr1fqbb7zhy8sqis1h9jgzl06x2s";
  meta = with pkgs.lib; {
    description = "";
    homepage = https://github.com/capatazlib/go-capataz;
    license = licenses.mit;
    maintainers = [ "Roman Gonzalez" ];
    platforms = platforms.linux ++ platforms.darwin;
  };
}

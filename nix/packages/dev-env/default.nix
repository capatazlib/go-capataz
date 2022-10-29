_system: _inputs: {
  buildEnv,
  gopls,
  go-tools,
  gotools,
  godef,
  revive,
  mkGoEnv,
}: let
  goEnv = mkGoEnv {
    pwd = ./../../..;
  };
in
  buildEnv {
    name = "dev-env";
    paths =
      goEnv.propagatedBuildInputs
      ++ [
        godef
        revive
        go-tools
        gotools
      ];
  }

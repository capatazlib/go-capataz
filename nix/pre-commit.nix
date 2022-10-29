system: {self, ...}: {
  lib,
  writeShellScript,
  revive,
}: {
  src = ./..;

  hooks = {
    alejandra.enable = true;
    shellcheck.enable = true;

    govet.enable = true;

    revive = {
      enable = true;
      name = "revive";
      description = "A linter for Go source code";
      entry = let
        cmdArgs =
          lib.concatStringsSep
          " "
          (
            builtins.concatLists [
              ["-set_exit_status"]
              ["-config .revive.toml"]
            ]
          );
        # revive works with both files and directories; however some lints
        # may fail (e.g. package-comment) if they run on an individual file
        # rather than a package/directory scope; given this let's get the
        # directories from each individual file.
        script = writeShellScript "precommit-revive" ''
          err=0
          for dir in $(echo "$@" | xargs -n1 dirname | sort -u); do
            ${revive}/bin/revive ${cmdArgs} ./"$dir"
            code="$?"
            if [[ "$err" -eq 0 ]]; then
               err="$code"
            fi
          done
          exit $err
        '';
      in
        builtins.toString script;
      files = "\\.go$";
      raw = {
        # to avoid multiple invocations of the same directory input, provide
        # all file names in a single run.
        require_serial = true;
      };
    };

    gotest = {
      enable = true;
      name = "gotest";
      description = "Run go tests";
      entry = let
        script = writeShellScript "precommit-gotest" ''
          set -e
          # find all directories that contain tests
          dirs=()
          for file in "$@"; do
            # either the file is a test
            if [[ "$file" = *_test.go ]]; then
              dirs+=("$(dirname "$file")")
              continue
            fi

            # or the file has an associated test
            filename="''${file%.go}"
            test_file="''${filename}_test.go"
            if [[ -f "$test_file"  ]]; then
              dirs+=("$(dirname "$test_file")")
              continue
            fi
          done

          # ensure we are not duplicating dir entries
          IFS=$'\n' sorted_dirs=($(sort -u <<<"''${dirs[*]}")); unset IFS

          # test each directory one by one
          for dir in "''${sorted_dirs[@]}"; do
              ${self.packages.${system}.dev-env}/bin/go test -timeout 10s -race -coverprofile=coverage.txt -covermode=atomic "./$dir"
          done
        '';
      in
        builtins.toString script;
      files = "\\.go$";
      raw = {
        # to avoid multiple invocations of the same directory input, provide
        # all file names in a single run.
        require_serial = true;
      };
    };

    staticcheck = {
      enable = true;
      name = "staticheck";
      description = "State of the art linter for the Go programming language";
      # staticheck works with directories.
      entry = let
        script = writeShellScript "precommit-staticcheck" ''
          err=0
          for dir in $(echo "$@" | xargs -n1 dirname | sort -u); do
            ${self.packages.${system}.dev-env}/bin/staticcheck ./"$dir"
            code="$?"
            if [[ "$err" -eq 0 ]]; then
               err="$code"
            fi
          done
          exit $err
        '';
      in
        builtins.toString script;
      files = "\\.go$";
      raw = {
        # to avoid multiple invocations of the same directory input, provide
        # all file names in a single run.
        require_serial = true;
      };
    };
  };
}

export PROJECT_PATH="$( dirname $( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd ) )"
export PROJECT_ID="$( gcloud config get-value project --format="value(.)" )"
export REGION="us-central1"

if [[ -z "$PROJECT_ID" ]]; then
  echo "Set a gcloud project (via gcloud config or CLOUDSDK_CORE_PROJECT)" >&2
  exit 1
fi

if ! python3 -c 'import cleverhans' &>/dev/null; then
  echo "Run in project environment" >&2
  exit 1
fi

prompt_deploy() {
  read -p "Deploy ${JOB_NAME} to ${PROJECT_ID} [y/N]? " -n 1 -r
  if [[ ! "$REPLY" =~ ^[Yy]$ ]]; then
    exit 1
  fi
  echo
}

build_dist() {
  rm -rf build dist
  python3 setup.py sdist
}

build_dovecotes() {
  pushd "$PROJECT_PATH/dovecotes" &>/dev/null
  build_dist &>/dev/null
  popd &>/dev/null
  ls "$PROJECT_PATH/dovecotes/dist/"dovecotes-*.tar.gz
}

deploy() {
  if [[ -n "${DEBUG:=}" ]]; then
    echo "$@"
    exit 0
  else
    exec "$@"
  fi
}

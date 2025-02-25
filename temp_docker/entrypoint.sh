#!/bin/bash

# Check if --api-root is provided
if [[ "$@" == *"--api-root"* ]]; then
  # Extract the value of --api-root
  api_root=$(echo "$@" | sed 's/.*--api-root=\([^ ]*\).*/\1/')

  # Remove --api-root from the arguments
  args=$(echo "$@" | sed 's/--api-root=[^ ]*//')

  # Add --api-definitions-path with the translated value
  args="$args --api-definitions-path=$api_root"
else
  # If --api-root is not provided, keep the arguments as is
  args="$@"
fi

echo "renamed api-root to api-definitions-path"
echo $args

# Check if --output is provided
if [[ "$@" == *"--output"* ]]; then
  # Extract the value of --output
  output=$(echo "$@" | sed 's/.*--output=\([^ ]*\).*/\1/')

  # Remove --output from the arguments
  args=$(echo "$args" | sed 's/--output=[^ ]*//')

  # Add --repository-path with the translated value
  args="$args --repository-path=$output"
  args="$args --generation-config-path=$output/generation_config.yaml"
else
  # If --output is not provided, keep the arguments as is
  args="$args"
fi


echo "renamed output to repository-path"
echo $args

# Check if --api-path is provided
if [[ "$@" == *"--api-path"* ]]; then
  # Extract the value of --api-path
  api-path=$(echo "$@" | sed 's/.*--api-path=\([^ ]*\).*/\1/')

  # Remove --api-path from the arguments
  args=$(echo "$args" | sed 's/--api-path=[^ ]*//')

else
  # If --output is not provided, keep the arguments as is
  args="$args"
fi

echo "removed api-path"
echo $args


echo "LOOK HERE LINE --------"

echo "Running Java generator with args: $args"

echo "python /src/library_generation/cli/entry_point.py \
$args --library-names=apigee-connect,asset "

python /src/library_generation/cli/entry_point.py \
$args --library-names=apigee-connect,asset 

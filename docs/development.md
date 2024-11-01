# Development

1. Prepare a Linux box running Docker.
2. Checkout this repository.

    ```bash
    git clone https://github.com/cybozu-go/nyamber
    ```

## Setup CLI tools

1. Install [aqua][].

    <https://aquaproj.github.io/docs/tutorial-basics/quick-start>

2. Install CLI tools.

    ```bash
    cd cybozu-go/nyamber
    aqua i -l
    ```

## Development & Debug

1. Launch local Kubernetes cluster.

    ```bash
    cd cybozu-go/nyamber
    make start
    ```

2. Start [Tilt][].

    ```bash
    cd cybozu-go/nyamber
    tilt up
    ```

3. Access: `http://localhost:10350/`
4. Stop the Kubernetes cluster.

    ```bash
    cd cybozu-go/nyamber
    make stop
    ```

[aqua]: https://aquaproj.github.io
[Tilt]: https://tilt.dev

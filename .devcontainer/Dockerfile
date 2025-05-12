FROM golang:1.24-bullseye

# Avoid warnings by switching to noninteractive
ENV DEBIAN_FRONTEND=noninteractive

# Install essential tools
RUN apt-get update \
    && apt-get -y install --no-install-recommends apt-utils 2>&1 \
    && apt-get -y install git curl wget lsb-release gnupg2 software-properties-common \
    build-essential pkg-config zip unzip jq bash-completion \
    && apt-get autoremove -y \
    && apt-get clean -y \
    && rm -rf /var/lib/apt/lists/*

# Install Go tools
RUN go install golang.org/x/tools/gopls@latest \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest \
    && go install github.com/securego/gosec/v2/cmd/gosec@latest \
    && go install golang.org/x/vuln/cmd/govulncheck@latest \
    && go install golang.org/x/tools/cmd/goimports@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install honnef.co/go/tools/cmd/staticcheck@latest \
    && go install github.com/fatih/gomodifytags@latest

# Install Python and pre-commit for git hooks
RUN apt-get update \
    && apt-get install -y python3 python3-pip \
    && pip3 install pre-commit \
    && apt-get autoremove -y \
    && apt-get clean -y \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME \
    && apt-get update \
    && apt-get install -y sudo \
    && echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Set up shell configuration for the non-root user
RUN echo 'source /usr/share/bash-completion/bash_completion' >> /home/$USERNAME/.bashrc \
    && echo 'export PATH=$PATH:/go/bin' >> /home/$USERNAME/.bashrc

# Switch back to dialog for any ad-hoc use of apt-get
ENV DEBIAN_FRONTEND=dialog

# Set working directory and user
WORKDIR /workspace
USER $USERNAME

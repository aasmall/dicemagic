FROM alpine
RUN apk add --no-cache curl && \
    curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin/kubectl && \
    opsys=linux && \
    curl -s https://api.github.com/repos/kubernetes-sigs/kustomize/releases/latest | grep browser_download | grep $opsys | cut -d '"' -f 4 | xargs curl -O -L && \
    tar -zxf $(find -type f -name "kustomize_*_${opsys}_amd64*"| head -n 1) --directory /usr/local/bin && \
    chmod u+x /usr/local/bin/kustomize && \
    rm $(find -type f -name "kustomize_*_${opsys}_amd64*"| head -n 1)
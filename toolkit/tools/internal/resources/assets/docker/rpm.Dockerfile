FROM mcr.microsoft.com/azurelinux/local_builder/base

# Tag the layers so we can clean up all the containers associated with a build directory
LABEL marinertoolchain=mockmockmock

RUN tdnf install -y rpm-build

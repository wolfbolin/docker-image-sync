# encoding=utf-8
import docker
import os.path
from common import *

docker_cli: docker.DockerClient = None


def docker_client():
    global docker_cli
    if docker_cli is None:
        docker_cli = docker.from_env()
    return docker_cli


def docker_pull_image(repo, name, tag, platform):
    # client = docker_client()
    # client.images.pull(os.path.join(repo, name), tag=tag, platform=platform)
    cmd = f"docker pull {os.path.join(repo, name)}:{tag} --platform={platform}"
    print_white(cmd)
    code = os.system(cmd)
    if code != 0:
        raise IOError


def docker_tag_image(source, target):
    client = docker_client()
    client.api.tag(source, target)


def docker_push_image(repo, name, tag):
    # client = docker_client()
    # client.images.push(os.path.join(repo, name), tag=tag)
    cmd = f"docker push {os.path.join(repo, name)}:{tag}"
    print_white(cmd)
    code = os.system(cmd)
    if code != 0:
        raise IOError


def docker_create_manifest(list_name, *manifest):
    cmd = f"docker manifest create --amend {list_name} " + " ".join(manifest)
    print_white(cmd)
    code = os.system(cmd)
    if code != 0:
        raise IOError


def docker_push_manifest(list_name):
    cmd = f"docker manifest push {list_name}"
    print_white(cmd)
    code = os.system(cmd)
    if code != 0:
        raise IOError


def docker_images_prune():
    cmd = f"docker image prune -af"
    print_white(cmd)
    code = os.system(cmd)
    if code != 0:
        raise IOError

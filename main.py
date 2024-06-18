# encoding=utf-8
import os.path
import pymysql
import pymysql.cursors
from common import *
from client import *
from repository import *


def main():
    mysql = pymysql.connect(host='ssas.wiolfi.cn', port=12806, user='root', passwd='', db='wiolfi')
    print("MySQL connect success")

    cursor = mysql.cursor(cursor=pymysql.cursors.DictCursor)
    cursor.execute("SELECT * FROM harbor_sync_rule")
    rules = cursor.fetchall()

    for rule in rules:
        registry = rule['registry']
        image_name = rule['name']
        tags_rule = rule['tags']

        print_green(f"Fetch image [{image_name}] tags from [{registry}]")
        harbor_tags = harbor_image_tags("library", image_name)
        for tag in docker_image_tags(image_name):
            if not re.match(tags_rule, tag["name"]):
                continue

            if tag['name'] in harbor_tags:
                print_yellow(f"Skip image tag {tag["name"]}. Docker image tag is existed.")
                continue

            if tag['media_type'].startswith("application/vnd.docker.distribution.manifest.v1"):
                print_yellow(f"Skip image tag {tag["name"]}. Docker image format is deprecated.")
                continue

            for i in range(3):
                docker_tag_sync(rule, tag)


def docker_tag_sync(sync_rule, tag_info):
    docker_images_prune()
    print_white(f"Recycle docker image usage")

    image_tag = tag_info['name']
    image_name = sync_rule['name']
    source_repo = os.path.join("hub.wiolfi.net", "docker")
    source_img = os.path.join(source_repo, image_name)
    source_tag = f"{source_img}:{image_tag}"

    target_repo = "hub.wiolfi.net/mirror"
    target_img = os.path.join(target_repo, image_name)
    target_tag = f"{target_img}:{image_tag}"

    manifest_repo = "hub.wiolfi.net/library"
    manifest_img = os.path.join(manifest_repo, image_name)
    manifest_tag = f"{manifest_img}:{image_tag}"

    print_blue(f"Sync image {source_tag} => {target_tag}")

    arch_images = []
    for arch in tag_info["images"]:
        if arch['architecture'] == 'unknown':
            continue

        target_arch_repo = os.path.join(target_repo, arch['os'], arch['architecture'])
        target_arch_img = os.path.join(target_arch_repo, image_name)
        target_arch_tag = f"{target_arch_img}:{image_tag}"

        platform = os.path.join(arch['os'], arch['architecture'])

        print_blue(f"Sync arch {source_tag} {arch['os']}/{arch['architecture']} => {target_arch_tag}")
        docker_pull_image(source_repo, image_name, image_tag, platform)
        docker_tag_image(source_tag, target_arch_tag)
        docker_push_image(target_arch_repo, image_name, image_tag)
        arch_images.append(target_arch_tag)

    print_blue(f"Create manifest {manifest_tag}")
    docker_create_manifest(manifest_tag, *arch_images)
    print_blue(f"Push manifest {manifest_tag}")
    docker_push_manifest(manifest_tag)


if __name__ == '__main__':
    main()

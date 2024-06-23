# encoding=utf-8
import os.path
import time

import pymysql
import pymysql.cursors
from common import *
from repository import *


def main():
    mysql = pymysql.connect(host='ssas.wiolfi.cn', port=12806, user='root', passwd=os.getenv("DB_PASSWD"), db='wiolfi')
    print("MySQL connect success")

    cursor = mysql.cursor(cursor=pymysql.cursors.DictCursor)
    cursor.execute("SELECT * FROM harbor_sync_rule")
    rules = cursor.fetchall()

    for rule in rules:
        registry = rule['registry']
        project = rule['project']
        image_name = rule['name']
        tags_rule = rule['tags']

        print_green(f"Fetch image [{image_name}] tags from [{registry}]")
        harbor_tags = harbor_image_tags("mirror", f"{project}%252F{image_name}")
        for tag in docker_image_tags(project, image_name):
            if not re.match(tags_rule, tag["name"]):
                continue

            if tag['name'] in harbor_tags:
                print_yellow(f"Skip image tag {tag["name"]}. Docker image tag is existed.")
                continue

            if tag['media_type'].startswith("application/vnd.docker.distribution.manifest.v1"):
                print_yellow(f"Skip image tag {tag["name"]}. Docker image format is deprecated.")
                continue

            for i in range(3):
                try:
                    docker_tag_sync(rule, tag)
                except ChildProcessError:
                    time.sleep(3)
                    continue


def docker_tag_sync(sync_rule, tag_info):
    image_tag = tag_info['name']
    image_name = sync_rule['name']

    source_repo = os.path.join("hub.wiolfi.net", "docker", sync_rule['project'])
    source_img = os.path.join(source_repo, image_name)
    source_tag = f"{source_img}:{image_tag}"

    target_repo = os.path.join("hub.wiolfi.net", "mirror", sync_rule['project'])
    target_img = os.path.join(target_repo, image_name)
    target_tag = f"{target_img}:{image_tag}"

    print_blue(f"Sync image {source_tag} => {target_tag}")
    cmd = f"skopeo copy --all --preserve-digests docker://{source_tag} docker://{target_tag}"
    run_shell(cmd)


def run_shell(cmd):
    print_white(cmd)
    code = os.system(cmd)
    if code != 0:
        raise ChildProcessError


if __name__ == '__main__':
    main()

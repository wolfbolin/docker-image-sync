# encoding=utf-8
import time
import os.path
import pymysql
import requests
from common import *
import pymysql.cursors
from repository import *


def main():
    session = requests.Session()

    mysql = pymysql.connect(host='ssas.wiolfi.cn', port=12806, user='root', passwd=os.getenv("DB_PASSWD"), db='wiolfi')
    print("MySQL connect success")

    cursor = mysql.cursor(cursor=pymysql.cursors.DictCursor)
    cursor.execute("SELECT * FROM harbor_sync_rule ORDER BY `id` DESC")
    rules = cursor.fetchall()

    img_count = [0, 0, 0, 0]  # Success, Failed, Exist, Skip
    for rule in rules:
        source_img = {
            "registry": rule['registry'],
            "project": rule['project'],
            "name": rule['name'],
            "tag": "",
        }
        target_img = {
            "registry": "hub.wiolfi.net",
            "project": f"mirror/{rule['project']}",
            "name": rule['name'],
            "tag": "",
        }

        print_green(f"Fetch image [{source_img['project']}/{source_img['name']}] "
                    f"from [{target_img['project']}/{target_img['name']}]")

        harbor_tags = harbor_image_tags(session, "mirror", f"{rule['project']}%252F{rule['name']}")

        if rule['tag'] != "" and rule['tag'] not in harbor_tags:
            source_img['tag'] = rule['tag']
            target_img['tag'] = rule['tag']
            docker_tag_sync(source_img, target_img)

        if rule['tags'] == "":
            continue

        docker_hub_tags = docker_image_tags(session, source_img['project'], source_img['name'])
        for remote_tag in docker_hub_tags:
            if not re.match(rule['tags'], remote_tag["name"]):
                img_count[3] += 1
                continue

            if remote_tag['media_type'].startswith("application/vnd.docker.distribution.manifest.v1"):
                print_yellow(f"Skip image tag {remote_tag["name"]}. Docker image format is deprecated.")
                img_count[3] += 1
                continue

            if remote_tag['name'] in harbor_tags:
                print_yellow(f"Skip image tag {remote_tag["name"]}. Docker image tag is existed.")
                img_count[2] += 1
                continue

            try:
                source_img['tag'] = remote_tag["name"]
                target_img['tag'] = remote_tag["name"]
                docker_tag_sync(source_img, target_img)
                img_count[0] += 1
            except ChildProcessError:
                img_count[1] += 1
                time.sleep(3)
                continue

    print("Sync harbor image finish: Success={} Failed={} Exist={} Skip={}".format(*img_count))


def docker_tag_sync(source_img, target_img):
    source_url = f"{os.path.join(source_img['registry'], source_img['project'], source_img['name'])}:{source_img['tag']}"
    target_url = f"{os.path.join(target_img['registry'], target_img['project'], target_img['name'])}:{target_img['tag']}"

    print_blue(f"Sync image {source_url} => {target_url}")
    cmd = f"skopeo copy --all --preserve-digests docker://{source_url} docker://{target_url}"
    run_shell(cmd)


def run_shell(cmd):
    print_white(cmd)
    code = os.system(cmd)
    if code != 0:
        raise ChildProcessError


if __name__ == '__main__':
    main()

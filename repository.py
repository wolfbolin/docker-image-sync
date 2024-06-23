# encoding=utf-8
import json

import requests

tags_cache = {}
proxies = {"http": "http://10.10.30.34:12821", "https": "http://10.10.30.34:12821"}


def docker_image_tags(project: str, name: str):
    if name in tags_cache.keys():
        return tags_cache[name]

    page = 1
    url = f"https://hub.docker.com/v2/namespaces/{project}/repositories/{name}/tags"

    tag_list = []
    while True:
        res = requests.get(url=url, params={"page": page, "page_size": 100}, proxies=proxies)
        print("GET", res.url)

        json_data = res.json()
        tag_list.extend(json_data["results"])

        if page * 100 >= json_data["count"]:
            break
        page += 1

    tags_cache[name] = tag_list
    return tag_list


def harbor_image_tags(project, name):
    page = 1
    tag_list = []
    url = f"https://hub.wiolfi.net/api/v2.0/projects/{project}/repositories/{name}/artifacts"
    while True:
        res = requests.get(url=url, params={"page": page, "page_size": 100}, proxies=proxies)
        print("GET", res.url)

        for item in res.json():
            if item['tags'] is not None:
                for tag in item['tags']:
                    tag_list.append(tag["name"])

        if len(res.json()) != 100:
            break
        page += 1

    return tag_list

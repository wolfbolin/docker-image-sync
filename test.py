import re

pattern = r'\d{2}\.04'
strings = ["22.04", "20.04", "abc 22.04 def", "xyz 20.04 uvw"]

for string in strings:
    if re.match(pattern, string):
        print(f'"{string}" 匹配正则表达式')
    else:
        print(f'"{string}" 不匹配正则表达式')

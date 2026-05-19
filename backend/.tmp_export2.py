from pathlib import Path
import re

root = Path(r'C:\Users\15034\GolandProjects\final-exam-savior\backend\internal\controller')
content = (root / 'controller.go').read_text('utf-8')

def upper_match(m):
    receiver = m.group(1)
    name = m.group(2)
    if name in ('ok', 'abort'): return m.group(0)
    upper_name = name[0].upper() + name[1:]
    return f"func ({receiver}) {upper_name}"

content = re.sub(r'func \((.*?)\) (\w+)', upper_match, content)
(root / 'controller.go').write_text(content, 'utf-8')

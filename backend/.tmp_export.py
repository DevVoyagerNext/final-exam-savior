from pathlib import Path
import re

root = Path(r'C:\Users\15034\GolandProjects\final-exam-savior\backend\internal\controller')
content = (root / 'controller.go').read_text('utf-8')

# The previous script might not have matched correctly if there's no space before `(c *Controller)`
# Or maybe the signature is `func (c *Controller) sendRegisterCode(c *gin.Context)` but parameter is named `c`
# Let's just blindly capitalize all func (c *Controller) xyz
def upper_match(m):
    name = m.group(1)
    if name in ('ok', 'abort'): return m.group(0)
    upper_name = name[0].upper() + name[1:]
    return f"func (c *Controller) {upper_name}"

content = re.sub(r'func \(c \*Controller\) (\w+)', upper_match, content)
(root / 'controller.go').write_text(content, 'utf-8')

# Also delete the old routes.go since we migrated it
(root / 'routes.go').unlink(missing_ok=True)

from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
from sklearn.linear_model import LogisticRegression

plt.rcParams.update({"pdf.fonttype": 42, "ps.fonttype": 42, "font.size": 8})
x = np.array([[0.0], [1.0], [2.0], [3.0]])
y = np.array([0, 0, 1, 1])
model = LogisticRegression(random_state=0).fit(x, y)
grid = np.linspace(0, 3, 80).reshape(-1, 1)

fig, ax = plt.subplots(figsize=(3.5, 2.2))
ax.plot(grid[:, 0], model.predict_proba(grid)[:, 1], color="#007E87", lw=1.5)
ax.scatter(x[:, 0], y, color="#222222", s=16, zorder=3)
ax.set(xlabel="Evidence", ylabel="Decision probability")
fig.tight_layout()
out = Path(__file__).parent
fig.savefig(out / "smoke.pdf")
fig.savefig(out / "smoke.png", dpi=300)
plt.close(fig)

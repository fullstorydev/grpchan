import plotly.express as px
import pandas as pd
import os


data = [['InProc', 27.44], ['ShmGrpc', 36.10], ['Http2', 368.6]]
df = pd.DataFrame(data, columns=['Channel', 'sec(µ)/op'])

df["e"] = [[2.0],[13],[13]]


fig = px.bar(df, x="Channel", y="sec(µ)/op", color="Channel", barmode="overlay", error_y="e")
    # error_y=dict(
    #             type='value', # value of error bar given in data coordinates
    #             array=[0.5488, 4.693, 46.918], #percent 2.0, 13, 13
    #             visible=True)
    #     )

if not os.path.exists("images"):
    os.mkdir("images")

# fig.show()
fig.write_image("images/channel_perf.png")

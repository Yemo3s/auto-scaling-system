#!/bin/bash

# 获取 service 的 NodePort 地址
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
NODE_PORT=30080

if [ -z "$NODE_IP" ]; then
    echo "无法获取节点 IP 地址"
    exit 1
fi

TARGET_URL="http://$NODE_IP:$NODE_PORT"
echo "开始对 nginx service ($TARGET_URL) 进行压力测试..."
echo "将持续发送请求 30 秒..."
echo "----------------------------------------"

# 测试连接是否正常
echo "测试连接..."
TEST_RESPONSE=$(curl -s -w "%{http_code}" "$TARGET_URL" -o /dev/null)
if [ "$TEST_RESPONSE" != "200" ]; then
    echo "错误: 无法连接到目标服务器，HTTP状态码: $TEST_RESPONSE"
    exit 1
fi
echo "连接测试成功！"
echo "----------------------------------------"

# 使用 curl 在后台连续发送请求
for i in {1..30}; do
    echo -n "第 $i 秒: "
    
    # 并发发送 100 个请求，但只显示前3个的结果
    success_count=0
    fail_count=0
    
    for j in {1..100}; do
        if [ $j -le 3 ]; then
            # 前3个请求显示详细结果
            response=$(curl -s -w "%{http_code}" "$TARGET_URL" -o /dev/null)
            if [ "$response" = "200" ]; then
                echo -n "✓ "
                ((success_count++))
            else
                echo -n "✗ "
                ((fail_count++))
            fi
        else
            # 其余请求在后台运行，只统计结果
            curl -s "$TARGET_URL" -o /dev/null &
            if [ $? -eq 0 ]; then
                ((success_count++))
            else
                ((fail_count++))
            fi
        fi
    done
    
    # 等待所有后台请求完成
    wait
    
    echo "  成功: $success_count, 失败: $fail_count"
    sleep 1
done

echo "----------------------------------------"
echo "压力测试完成"
echo "提示: 运行 'kubectl top pods' 查看资源使用情况" 